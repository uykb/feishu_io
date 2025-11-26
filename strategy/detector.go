package strategy

import (
	"binance-monitor/models"
	"log"
	"sync"
	"time"
)

// KlineHistory K线历史存储
type KlineHistory struct {
	mu        sync.RWMutex
	klines    map[string][]models.KlineData // symbol -> []KlineData
	maxLength int                           // 最大历史记录长度
}

// OIStorage 持仓量存储
type OIStorage struct {
	mu  sync.RWMutex
	ois map[string]float64 // symbol -> open interest
}

// RatioStorage 多空比存储
type RatioStorage struct {
	mu     sync.RWMutex
	ratios map[string]float64 // symbol -> long/short ratio
}

// SignalDetector 信号检测器
type SignalDetector struct {
	klineHistory   *KlineHistory
	oiStorage      *OIStorage
	ratioStorage   *RatioStorage
	oiThreshold    float64
	priceThreshold float64
	signalCh       chan models.Signal
	atrPeriod      int
	alertTimestamps map[string][]time.Time
	alertMu         sync.Mutex
}

// NewSignalDetector 创建信号检测器
func NewSignalDetector(oiThreshold, priceThreshold float64, signalCh chan models.Signal) *SignalDetector {
	return &SignalDetector{
		klineHistory: &KlineHistory{
			klines:    make(map[string][]models.KlineData),
			maxLength: 20, // 存储足够的K线用于计算ATR(14)等指标
		},
		oiStorage: &OIStorage{
			ois: make(map[string]float64),
		},
		ratioStorage: &RatioStorage{
			ratios: make(map[string]float64),
		},
		oiThreshold:    oiThreshold,
		priceThreshold: priceThreshold,
		signalCh:       signalCh,
		atrPeriod:      14, // ATR 计算周期
		alertTimestamps: make(map[string][]time.Time),
	}
}

// ProcessKlineData 处理K线数据
func (sd *SignalDetector) ProcessKlineData(klineDataCh <-chan models.KlineData) {
	for kline := range klineDataCh {
		sd.klineHistory.mu.Lock()
		history := sd.klineHistory.klines[kline.Symbol]
		history = append(history, kline)
		if len(history) > sd.klineHistory.maxLength {
			history = history[1:] // 维持最大长度
		}
		sd.klineHistory.klines[kline.Symbol] = history

		// 需要至少两条K线来计算价格变化
		if len(history) < 2 {
			sd.klineHistory.mu.Unlock()
			continue
		}
		previousPrice := history[len(history)-2].Close
		sd.klineHistory.mu.Unlock() // checkSignal会再次锁定

		// 检查是否触发信号
		sd.checkSignal(kline, previousPrice)
	}
}

// ProcessOIData 处理持仓量数据
func (sd *SignalDetector) ProcessOIData(oiDataCh <-chan models.OIData) {
	for oiData := range oiDataCh {
		sd.oiStorage.mu.Lock()
		sd.oiStorage.ois[oiData.Symbol] = oiData.OpenInterest
		sd.oiStorage.mu.Unlock()
	}
}

// ProcessRatioData 处理多空比数据
func (sd *SignalDetector) ProcessRatioData(ratioDataCh <-chan models.LongShortRatioData) {
	for ratioData := range ratioDataCh {
		sd.ratioStorage.mu.Lock()
		sd.ratioStorage.ratios[ratioData.Symbol] = ratioData.LongShortRatio
		sd.ratioStorage.mu.Unlock()
	}
}

// checkSignal 检查是否触发信号
func (sd *SignalDetector) checkSignal(currentKline models.KlineData, previousPrice float64) {
	// 获取当前OI和上一次OI
	sd.oiStorage.mu.RLock()
	currentOI, oiExists := sd.oiStorage.ois[currentKline.Symbol]
	sd.oiStorage.mu.RUnlock()

	if !oiExists || previousPrice == 0 {
		return
	}

	// 计算价格变化率
	currentPrice := currentKline.Close
	priceChange := ((currentPrice - previousPrice) / previousPrice) * 100

	// 由于OI是轮询获取的，我们需要存储历史OI来计算变化率
	// 简化处理：使用内存缓存存储上一个周期的OI
	previousOI := sd.getPreviousOI(currentKline.Symbol)
	if previousOI == 0 {
		sd.savePreviousOI(currentKline.Symbol, currentOI)
		return
	}

	// 计算OI变化率
	oiChange := ((currentOI - previousOI) / previousOI) * 100

	// 检查四种信号类型
	var signalType models.SignalType
	var matched bool

	if oiChange > sd.oiThreshold && priceChange > sd.priceThreshold {
		// OI↑ + Price↑ = 看涨突破
		signalType = models.BullishBreakout
		matched = true
	} else if oiChange > sd.oiThreshold && priceChange < -sd.priceThreshold {
		// OI↑ + Price↓ = 看跌动量
		signalType = models.BearishMomentum
		matched = true
	}

	if matched {
		// -- 多空比过滤 --
		sd.ratioStorage.mu.RLock()
		ratio, ratioExists := sd.ratioStorage.ratios[currentKline.Symbol]
		sd.ratioStorage.mu.RUnlock()

		if !ratioExists {
			return // 如果没有多空比数据，则不过滤，直接返回
		}

		// 应用过滤逻辑
		if signalType == models.BullishBreakout && ratio >= 1 {
			log.Printf("过滤信号: %s 看涨信号，但多空比(%.2f) >= 1", currentKline.Symbol, ratio)
			return
		}
		if signalType == models.BearishMomentum && ratio <= 1 {
			log.Printf("过滤信号: %s 看跌信号，但多空比(%.2f) <= 1", currentKline.Symbol, ratio)
			return
		}


		// -- ATR, Stop-Loss, and Quantity Calculation --
		sd.klineHistory.mu.RLock()
		klineHistory, historyExists := sd.klineHistory.klines[currentKline.Symbol]
		sd.klineHistory.mu.RUnlock()

		var atr, stopLoss, quantity float64
		if historyExists {
			atr = CalculateATR(klineHistory, sd.atrPeriod)

			// -- 24小时内信号计数 --
			alertCount := sd.getAlertCount(currentKline.Symbol)

			if atr > 0 {
				stopLossDistance := 1.5 * atr
				// 根据信号类型计算止损和仓位
				if signalType == models.BullishBreakout || signalType == models.PossibleFakeout { // 看涨/做多
					stopLoss = currentPrice - stopLossDistance
					quantity = 1 / stopLossDistance // 固定1 USDT风险
				} else { // 看跌/做空
					stopLoss = currentPrice + stopLossDistance
					quantity = 1 / stopLossDistance // 固定1 USDT风险
				}
			}
		}

		signal := models.Signal{
			Symbol:       currentKline.Symbol,
			SignalType:   signalType,
			PriceChange:  priceChange,
			OIChange:     oiChange,
			CurrentPrice: currentPrice,
			CurrentOI:    currentOI,
			Timestamp:    currentKline.Timestamp,
			ATR:          atr,
			StopLoss:     stopLoss,
			Quantity:     quantity,
			AlertsIn24h:  alertCount,
			LongShortRatio: ratio,
		}

		select {
		case sd.signalCh <- signal:
			log.Printf("触发信号: %s - %s (OI: %.2f%%, P: %.2f%%, ATR: %.4f, SL: %.4f, Qty: %.2f)",
				currentKline.Symbol, signalType.String(), oiChange, priceChange, atr, stopLoss, quantity)
		default:
			log.Printf("信号通道已满，丢弃 %s 的信号", currentKline.Symbol)
		}

		// 更新上一个OI
		sd.savePreviousOI(currentKline.Symbol, currentOI)
	}
}

// 简化的OI历史存储（实际应用中可以使用更复杂的结构）
var previousOIMap = make(map[string]float64)
var previousOIMu sync.RWMutex

func (sd *SignalDetector) getPreviousOI(symbol string) float64 {
	previousOIMu.RLock()
	defer previousOIMu.RUnlock()
	return previousOIMap[symbol]
}

func (sd *SignalDetector) savePreviousOI(symbol string, oi float64) {
	previousOIMu.Lock()
	defer previousOIMu.Unlock()
	previousOIMap[symbol] = oi
}

// getAlertCount 获取并更新24小时内的信号计数
func (sd *SignalDetector) getAlertCount(symbol string) int {
	sd.alertMu.Lock()
	defer sd.alertMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-24 * time.Hour)

	// 获取当前交易对的时间戳列表
	timestamps := sd.alertTimestamps[symbol]

	// 过滤掉旧的时间戳
	var recentTimestamps []time.Time
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			recentTimestamps = append(recentTimestamps, ts)
		}
	}

	// 添加当前时间戳
	recentTimestamps = append(recentTimestamps, now)

	// 更新存储
	sd.alertTimestamps[symbol] = recentTimestamps

	return len(recentTimestamps)
}
