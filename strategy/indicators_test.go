package strategy

import (
	"binance-monitor/models"
	"math"
	"testing"
)

func TestCalculateATR(t *testing.T) {
	klines := []models.KlineData{
		{High: 10, Low: 8, Close: 9},
		{High: 11, Low: 9, Close: 10},
		{High: 12, Low: 10, Close: 11},
	}
	// TR1: 10-8=2, |10-0|=10 (skip first prevClose?) - trueRange needs prev.Close
	// Implementation:
	// i=1: TR = Max(11-9, |11-9|=2, |9-9|=0) = 2
	// i=2: TR = Max(12-10, |12-10|=2, |10-10|=0) = 2
	// Sum = 4, Len = 2. ATR = 2.

	atr := CalculateATR(klines, 2)
	if atr != 2.0 {
		t.Errorf("Expected ATR 2.0, got %f", atr)
	}
}

func TestWilderSmoothing(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	period := 3
	// SMA of first 3: (1+2+3)/3 = 2
	// Next: (2*(3-1) + 4) / 3 = (4+4)/3 = 8/3 = 2.666
	// Next: (2.666*(2) + 5) / 3 = (5.333 + 5)/3 = 3.444

	smoothed := WilderSmoothing(data, period)
	
	if len(smoothed) != 5 {
		t.Errorf("Expected length 5, got %d", len(smoothed))
	}
	
	expected := 8.0/3.0
	if math.Abs(smoothed[3] - expected) > 0.001 {
		t.Errorf("Expected %f, got %f", expected, smoothed[3])
	}
}
