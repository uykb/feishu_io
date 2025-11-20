package strategy

import (
	"log"
	"sync"
	"time"

	"binance-monitor/models"
)

// PriceStorage 价格存储
type PriceStorage struct {
	mu     sync.RWMutex
	prices map[string]float64 // symbol -> close price
}

// OIStorage 持仓量存储
type OIStorage struct {
	mu  sync.RWMutex
	ois map[string]float64 // symbol -> open interest
}

// SignalDetector 信号检测器
type SignalDetector struct {
	priceStorage   *PriceStorage
	oiStorage      *OIStorage
	oiThreshold    float64
	priceThreshold float64
	signalCh       chan models.Signal
}

// NewSignalDetector 创建信号检测器
func NewSignalDetector(oiThreshold, priceThreshold float64, signalCh chan models.Signal) *SignalDetector {
	return &SignalDetector{
		priceStorage: &PriceStorage{
			prices: make(map[string]float64),
		},
		oiStorage: &OIStorage{
			ois: make(map[string]float64),
		},
		oiThreshold:    oiThreshold,
		priceThreshold: priceThreshold,
		signalCh:       signalCh,
	}
}

// ProcessKlineData 处理K线数据
func (sd *SignalDetector) ProcessKlineData(klineDataCh <-chan models.KlineData) {
	for kline := range klineDataCh {
		sd.priceStorage.mu.Lock()
		previousPrice, exists := sd.priceStorage.prices[kline.Symbol]
		sd.priceStorage.prices[kline.Symbol] = kline.Close
		sd.priceStorage.mu.Unlock()

		if !exists {
			continue
		}

		// 检查是否触发信号
		sd.checkSignal(kline.Symbol, previousPrice, kline.Close, kline.Timestamp)
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
func (sd *SignalDetector) checkSignal(symbol string, previousPrice, currentPrice float64, timestamp time.Time) {
	// 获取当前OI和上一次OI
	sd.oiStorage.mu.RLock()
	currentOI, oiExists := sd.oiStorage.ois[symbol]
	sd.oiStorage.mu.RUnlock()

	if !oiExists || previousPrice == 0 {
		return
	}

	// 计算价格变化率
	priceChange := ((currentPrice - previousPrice) / previousPrice) * 100

	// 由于OI是轮询获取的，我们需要存储历史OI来计算变化率
	// 简化处理：使用内存缓存存储上一个周期的OI
	previousOI := sd.getPreviousOI(symbol)
	if previousOI == 0 {
		sd.savePreviousOI(symbol, currentOI)
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
	} else if oiChange < -sd.oiThreshold && priceChange > sd.priceThreshold {
		// OI↓ + Price↑ = 可能假突破
		signalType = models.PossibleFakeout
		matched = true
	} else if oiChange < -sd.oiThreshold && priceChange < -sd.priceThreshold {
		// OI↓ + Price↓ = 市场收缩
		signalType = models.MarketContraction
		matched = true
	}

	if matched {
		signal := models.Signal{
			Symbol:       symbol,
			SignalType:   signalType,
			PriceChange:  priceChange,
			OIChange:     oiChange,
			CurrentPrice: currentPrice,
			CurrentOI:    currentOI,
			Timestamp:    timestamp,
		}

		select {
		case sd.signalCh <- signal:
			log.Printf("触发信号: %s - %s (OI变化: %.2f%%, 价格变化: %.2f%%)",
				symbol, signalType.String(), oiChange, priceChange)
		default:
			log.Printf("信号通道已满，丢弃 %s 的信号", symbol)
		}

		// 更新上一个OI
		sd.savePreviousOI(symbol, currentOI)
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
