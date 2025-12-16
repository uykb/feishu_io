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

// DetectorConfig 信号检测器配置
type DetectorConfig struct {
	OIThreshold             float64
	PriceThreshold          float64
	ADXThreshold            float64
	ATRPeriod               int
	ADXPeriod               int
	StopLossMultiplier      float64
	RiskAmount              float64
	BullishBreakoutWeight   float64
	BearishMomentumWeight   float64
	PossibleFakeoutWeight   float64
	MarketContractionWeight float64
}

// SignalDetector 信号检测器
type SignalDetector struct {
	config         DetectorConfig
	klineHistory   *KlineHistory
	oiStorage      *OIStorage
	signalCh       chan models.Signal
	alertTimestamps map[string][]time.Time
	alertMu         sync.Mutex
}

// NewSignalDetector 创建信号检测器
func NewSignalDetector(config DetectorConfig, signalCh chan models.Signal) *SignalDetector {
	return &SignalDetector{
		config: config,
		klineHistory: &KlineHistory{
			klines:    make(map[string][]models.KlineData),
			maxLength: config.ADXPeriod * 2 + 5, // 动态调整历史长度
		},
		oiStorage: &OIStorage{
			ois: make(map[string]float64),
		},
		signalCh:       signalCh,
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
	var strengthScore float64 // 新增信号强度评分

	// 获取历史K线数据用于ADX计算
	sd.klineHistory.mu.RLock()
	klineHistory, historyExists := sd.klineHistory.klines[currentKline.Symbol]
	sd.klineHistory.mu.RUnlock()

	var atr, stopLoss, quantity, adx float64
	var alertCount int

	if historyExists {
		adx = CalculateADX(klineHistory, sd.config.ADXPeriod)
	}

	// 信号检测逻辑
	if oiChange > sd.config.OIThreshold && priceChange > sd.config.PriceThreshold {
		// OI↑ + Price↑ = 看涨突破
		signalType = models.BullishBreakout
		matched = true
		strengthScore = (oiChange/sd.config.OIThreshold) * (priceChange/sd.config.PriceThreshold) * sd.config.BullishBreakoutWeight
	} else if oiChange > sd.config.OIThreshold && priceChange < -sd.config.PriceThreshold {
		// OI↑ + Price↓ = 看跌动量
		signalType = models.BearishMomentum
		matched = true
		strengthScore = (oiChange/sd.config.OIThreshold) * (math.Abs(priceChange)/sd.config.PriceThreshold) * sd.config.BearishMomentumWeight
	} else if oiChange < -sd.config.OIThreshold && priceChange > sd.config.PriceThreshold {
		// OI↓ + Price↑ = 可能的假突破
		signalType = models.PossibleFakeout
		matched = true
		strengthScore = (math.Abs(oiChange)/sd.config.OIThreshold) * (priceChange/sd.config.PriceThreshold) * sd.config.PossibleFakeoutWeight
	} else if oiChange < -sd.config.OIThreshold && priceChange < -sd.config.PriceThreshold {
		// OI↓ + Price↓ = 市场收缩
		signalType = models.MarketContraction
		matched = true
		strengthScore = (math.Abs(oiChange)/sd.config.OIThreshold) * (math.Abs(priceChange)/sd.config.PriceThreshold) * sd.config.MarketContractionWeight
	}

	if matched {
		// 检查ADX，ADX低于阈值则过滤信号
		if adx < sd.config.ADXThreshold {
			log.Printf("过滤信号: %s %s ADX (%.2f) < 阈值 (%.2f)", currentKline.Symbol, signalType.String(), adx, sd.config.ADXThreshold)
			return
		}
		// 根据ADX增强信号强度评分 (如果ADX高于阈值，则视为趋势更强)
		if adx > sd.config.ADXThreshold && strengthScore > 0 {
			strengthScore *= (1 + (adx-sd.config.ADXThreshold)/sd.config.ADXThreshold)
		}
		// 确保strengthScore不为负且有下限
		if strengthScore < 0 { strengthScore = 0 }


		// -- ATR, Stop-Loss, and Quantity Calculation --
		if historyExists {
			atr = CalculateATR(klineHistory, sd.config.ATRPeriod)

			// -- 24小时内信号计数 --
			alertCount = sd.getAlertCount(currentKline.Symbol)

			if atr > 0 {
				stopLossDistance := sd.config.StopLossMultiplier * atr
				// 根据信号类型计算止损和仓位
				if signalType == models.BullishBreakout || signalType == models.PossibleFakeout { // 看涨/做多
					stopLoss = currentPrice - stopLossDistance
					quantity = sd.config.RiskAmount / stopLossDistance // 固定风险
				} else { // 看跌/做空
					stopLoss = currentPrice + stopLossDistance
					quantity = sd.config.RiskAmount / stopLossDistance // 固定风险
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
			ADX:          adx,
			StrengthScore: strengthScore,
		}

		select {
		case sd.signalCh <- signal:
			log.Printf("触发信号: %s - %s (OI: %.2f%%, P: %.2f%%, ATR: %.4f, SL: %.4f, Qty: %.2f, Strength: %.2f)",
				currentKline.Symbol, signalType.String(), oiChange, priceChange, atr, stopLoss, quantity, strengthScore)
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
