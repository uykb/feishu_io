package strategy

import (
	"binance-monitor/models"
	"log"
	"sync"
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

// SignalDetector 信号检测器
type SignalDetector struct {
	klineHistory   *KlineHistory
	oiStorage      *OIStorage
	oiThreshold    float64
	priceThreshold float64
	signalCh       chan models.Signal
	atrPeriod      int
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
		oiThreshold:    oiThreshold,
		priceThreshold: priceThreshold,
		signalCh:       signalCh,
		atrPeriod:      14, // ATR 计算周期
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
		// -- ATR, Stop-Loss, and Quantity Calculation --
		sd.klineHistory.mu.RLock()
		klineHistory, historyExists := sd.klineHistory.klines[currentKline.Symbol]
		sd.klineHistory.mu.RUnlock()

		var atr, stopLoss, quantity float64
		if historyExists {
			atr = CalculateATR(klineHistory, sd.atrPeriod)

			if atr > 0 {
				stopLossDistance := 2 * atr
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
