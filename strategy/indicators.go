package strategy

import (
	"binance-monitor/models"
	"math"
)

// CalculateATR 计算平均真实波幅 (ATR)
func CalculateATR(klines []models.KlineData, period int) float64 {
	if len(klines) < period {
		return 0
	}

	var trSum float64
	for i := 1; i < len(klines); i++ {
		trSum += trueRange(klines[i], klines[i-1])
	}

	return trSum / float64(len(klines)-1)
}

func trueRange(current, previous models.KlineData) float64 {
	highLow := current.High - current.Low
	highClose := math.Abs(current.High - previous.Close)
	lowClose := math.Abs(current.Low - previous.Close)
	return math.Max(highLow, math.Max(highClose, lowClose))
}

// CalculateADX 计算平均方向指数 (ADX)
func CalculateADX(klines []models.KlineData, period int) float64 {
	if len(klines) < period*2 { // Need enough data for smoothing
		return 0
	}

	var plusDMs, minusDMs, trs []float64

	for i := 1; i < len(klines); i++ {
		high, low := klines[i].High, klines[i].Low
		prevHigh, prevLow := klines[i-1].High, klines[i-1].Low

		upMove := high - prevHigh
		downMove := prevLow - low

		var plusDM, minusDM float64
		if upMove > downMove && upMove > 0 {
			plusDM = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM = downMove
		}

		plusDMs = append(plusDMs, plusDM)
		minusDMs = append(minusDMs, minusDM)
		trs = append(trs, trueRange(klines[i], klines[i-1]))
	}

	smoothedPlusDM := WilderSmoothing(plusDMs, period)
	smoothedMinusDM := WilderSmoothing(minusDMs, period)
	smoothedTR := WilderSmoothing(trs, period)

	var dxs []float64
	for i := range smoothedTR {
		if smoothedTR[i] == 0 {
			dxs = append(dxs, 0)
			continue
		}
		plusDI := 100 * (smoothedPlusDM[i] / smoothedTR[i])
		minusDI := 100 * (smoothedMinusDM[i] / smoothedTR[i])

		if plusDI+minusDI == 0 {
			dxs = append(dxs, 0)
			continue
		}

		dx := 100 * math.Abs(plusDI-minusDI) / (plusDI + minusDI)
		dxs = append(dxs, dx)
	}

	adxValues := WilderSmoothing(dxs, period)

	if len(adxValues) > 0 {
		return adxValues[len(adxValues)-1]
	}

	return 0
}

// WilderSmoothing Wilder's Smoothing (similar to EMA)
func WilderSmoothing(data []float64, period int) []float64 {
	smoothed := make([]float64, len(data))
	if len(data) == 0 {
		return smoothed
	}

	// Initial SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		if i >= len(data) {
			break
		}
		sum += data[i]
	}
	smoothed[period-1] = sum / float64(period)

	// Wilder's smoothing for the rest
	for i := period; i < len(data); i++ {
		smoothed[i] = (smoothed[i-1]*float64(period-1) + data[i]) / float64(period)
	}
	return smoothed
}
