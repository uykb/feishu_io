package strategy

import (
	"binance-monitor/models"
	"testing"
	"time"
)

func TestSignalDetector_CheckSignal(t *testing.T) {
	config := DetectorConfig{
		OIThreshold:           5.0,
		PriceThreshold:        2.0,
		ADXThreshold:          20.0,
		ATRPeriod:             14,
		ADXPeriod:             14,
		StopLossMultiplier:    1.5,
		RiskAmount:            100.0,
		BullishBreakoutWeight: 1.0,
		BearishMomentumWeight: 1.0,
	}

	signalCh := make(chan models.Signal, 10)
	detector := NewSignalDetector(config, signalCh)

	// Mock data
	symbol := "BTCUSDT"
	now := time.Now()

	// 1. Simulate Bullish Breakout
	// Increase price by 3% (threshold 2%)
	// Increase OI by 6% (threshold 5%)

	// Setup initial state (previous price and OI)
	previousPrice := 10000.0
	previousOI := 1000.0
	detector.savePreviousOI(symbol, previousOI)
	detector.oiStorage.mu.Lock()
	detector.oiStorage.ois[symbol] = 1060.0 // +6%
	detector.oiStorage.mu.Unlock()

	// Fill Kline history for ADX/ATR (Need > 30 klines)
	detector.klineHistory.mu.Lock()
	history := make([]models.KlineData, 0)
	for i := 0; i < 40; i++ {
		history = append(history, models.KlineData{
			Symbol:    symbol,
			Timestamp: now.Add(time.Duration(-40+i) * 15 * time.Minute),
			Open:      10000,
			High:      10100,
			Low:       9900,
			Close:     10000,
		})
	}
	detector.klineHistory.klines[symbol] = history
	detector.klineHistory.mu.Unlock()

	currentKline := models.KlineData{
		Symbol:    symbol,
		Timestamp: now,
		Open:      10000,
		High:      10350,
		Low:       10000,
		Close:     10300, // +3%
	}

	// Trigger check
	detector.checkSignal(currentKline, previousPrice)

	// Verify signal
	select {
	case signal := <-signalCh:
		if signal.SignalType != models.BullishBreakout {
			t.Errorf("Expected BullishBreakout, got %v", signal.SignalType)
		}
		if signal.Symbol != symbol {
			t.Errorf("Expected symbol %s, got %s", symbol, signal.Symbol)
		}
		if signal.StopLoss == 0 {
			t.Error("Expected StopLoss to be calculated")
		}
		expectedQty := config.RiskAmount / (currentKline.Close - signal.StopLoss)
		// Allow small float error
		if signal.Quantity < expectedQty*0.99 || signal.Quantity > expectedQty*1.01 {
			t.Errorf("Expected Quantity around %.2f, got %.2f", expectedQty, signal.Quantity)
		}
	default:
		t.Error("Expected signal, got none")
	}
}

func TestSignalDetector_FilterLowADX(t *testing.T) {
	config := DetectorConfig{
		OIThreshold:    5.0,
		PriceThreshold: 2.0,
		ADXThreshold:   50.0, // High threshold to force filter
		ATRPeriod:      14,
		ADXPeriod:      14,
	}

	signalCh := make(chan models.Signal, 10)
	detector := NewSignalDetector(config, signalCh)

	symbol := "ETHUSDT"
	now := time.Now()

	previousPrice := 2000.0
	previousOI := 5000.0
	detector.savePreviousOI(symbol, previousOI)
	detector.oiStorage.mu.Lock()
	detector.oiStorage.ois[symbol] = 5300.0 // +6%
	detector.oiStorage.mu.Unlock()

	// Fill Kline history with low volatility (Low ADX)
	detector.klineHistory.mu.Lock()
	history := make([]models.KlineData, 0)
	for i := 0; i < 40; i++ {
		history = append(history, models.KlineData{
			Symbol:    symbol,
			Timestamp: now.Add(time.Duration(-40+i) * 15 * time.Minute),
			Open:      2000,
			High:      2001,
			Low:       1999,
			Close:     2000,
		})
	}
	detector.klineHistory.klines[symbol] = history
	detector.klineHistory.mu.Unlock()

	currentKline := models.KlineData{
		Symbol:    symbol,
		Timestamp: now,
		Open:      2000,
		High:      2060,
		Low:       2000,
		Close:     2060, // +3%
	}

	detector.checkSignal(currentKline, previousPrice)

	select {
	case <-signalCh:
		t.Error("Expected signal to be filtered due to low ADX, but received one")
	default:
		// Success
	}
}
