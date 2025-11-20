package strategy

import (
	"binance-monitor/models"
	"math"
)

// CalculateTrueRange 计算单个K线的真实波幅 (TR)
func CalculateTrueRange(current, previous models.KlineData) float64 {
	highLow := current.High - current.Low
	highPrevClose := math.Abs(current.High - previous.Close)
	lowPrevClose := math.Abs(current.Low - previous.Close)
	return math.Max(highLow, math.Max(highPrevClose, lowPrevClose))
}

// CalculateATR 计算平均真实波幅 (ATR)
// klines: K线历史数据，应按时间从远到近排序
// period: ATR计算周期，例如 14
func CalculateATR(klines []models.KlineData, period int) float64 {
	if len(klines) < period+1 {
		return 0 // 数据不足
	}

	trValues := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		tr := CalculateTrueRange(klines[i], klines[i-1])
		trValues = append(trValues, tr)
	}

	if len(trValues) < period {
		return 0 // TR数据不足
	}

	// 计算第一个 ATR (前 'period' 个 TR 的简单平均)
	sumTR := 0.0
	// 我们从TR数组的末尾开始取最新的 'period' 个值
	recentTRs := trValues[len(trValues)-period:]
	for _, tr := range recentTRs {
		sumTR += tr
	}
	atr := sumTR / float64(period)

	return atr
}
