package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	LarkWebhookURL string
	OIThreshold    float64
	PriceThreshold float64
	CheckInterval  int
	MinCheckInterval int
	ADXThreshold   float64
	// Strategy Configuration
	ATRPeriod          int
	ADXPeriod          int
	StopLossMultiplier float64
	RiskAmount         float64
	// Infrastructure Configuration
	LarkTimeout int
	Socks5Proxy string
	BullishBreakoutWeight float64
	BearishMomentumWeight float64
	PossibleFakeoutWeight float64
	MarketContractionWeight float64
	// HYPE Strategy Configuration
	HypeSymbol            string
	HypeOIStopThreshold   float64
	HypeFRExtremeThreshold float64
	HypeFRRecoveryThreshold float64
	HypeHigherLowPct      float64
	HypeSqueezePricePct   float64
	HypeSqueezeOIDeclinePct float64
	HypeCooldownMinutes   int
	HypeLookbackKlines    int
	HypeFundingInterval   int
}

func Load() *Config {
	// 尝试加载 .env 文件（Docker中不需要）
	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env file not found, using environment variables (normal for Docker)")
	}

	oiThreshold, _ := strconv.ParseFloat(getEnv("OI_THRESHOLD", "5.0"), 64)
	priceThreshold, _ := strconv.ParseFloat(getEnv("PRICE_THRESHOLD", "2.0"), 64)
	checkInterval, _ := strconv.Atoi(getEnv("CHECK_INTERVAL", "60"))
	minCheckInterval, _ := strconv.Atoi(getEnv("MIN_CHECK_INTERVAL", "10"))
	adxThreshold, _ := strconv.ParseFloat(getEnv("ADX_THRESHOLD", "20.0"), 64)
	atrPeriod, _ := strconv.Atoi(getEnv("ATR_PERIOD", "14"))
	adxPeriod, _ := strconv.Atoi(getEnv("ADX_PERIOD", "14"))
	stopLossMultiplier, _ := strconv.ParseFloat(getEnv("STOP_LOSS_MULTIPLIER", "1.5"), 64)
	riskAmount, _ := strconv.ParseFloat(getEnv("RISK_AMOUNT", "1.0"), 64)
	larkTimeout, _ := strconv.Atoi(getEnv("LARK_TIMEOUT", "10"))
	bullishBreakoutWeight, _ := strconv.ParseFloat(getEnv("BULLISH_BREAKOUT_WEIGHT", "1.0"), 64)
	bearishMomentumWeight, _ := strconv.ParseFloat(getEnv("BEARISH_MOMENTUM_WEIGHT", "1.0"), 64)
	possibleFakeoutWeight, _ := strconv.ParseFloat(getEnv("POSSIBLE_FAKEOUT_WEIGHT", "1.0"), 64)
	marketContractionWeight, _ := strconv.ParseFloat(getEnv("MARKET_CONTRACTION_WEIGHT", "1.0"), 64)
	hypeOIStopThreshold, _ := strconv.ParseFloat(getEnv("HYPE_OI_STOP_THRESHOLD", "-0.15"), 64)
	hypeFRExtremeThreshold, _ := strconv.ParseFloat(getEnv("HYPE_FR_EXTREME_THRESHOLD", "-0.0005"), 64)
	hypeFRRecoveryThreshold, _ := strconv.ParseFloat(getEnv("HYPE_FR_RECOVERY_THRESHOLD", "0.3"), 64)
	hypeHigherLowPct, _ := strconv.ParseFloat(getEnv("HYPE_HIGHER_LOW_PCT", "0.3"), 64)
	hypeSqueezePricePct, _ := strconv.ParseFloat(getEnv("HYPE_SQUEEZE_PRICE_PCT", "0.5"), 64)
	hypeSqueezeOIDeclinePct, _ := strconv.ParseFloat(getEnv("HYPE_SQUEEZE_OI_DECLINE_PCT", "0.05"), 64)
	hypeCooldownMinutes, _ := strconv.Atoi(getEnv("HYPE_COOLDOWN_MINUTES", "15"))
	hypeLookbackKlines, _ := strconv.Atoi(getEnv("HYPE_LOOKBACK_KLINES", "12"))
	hypeFundingInterval, _ := strconv.Atoi(getEnv("HYPE_FUNDING_INTERVAL", "30"))

	return &Config{
		LarkWebhookURL: getEnv("LARK_WEBHOOK_URL", ""),
		OIThreshold:    oiThreshold,
		PriceThreshold: priceThreshold,
		CheckInterval:  checkInterval,
		MinCheckInterval: minCheckInterval,
		ADXThreshold:   adxThreshold,
		ATRPeriod:          atrPeriod,
		ADXPeriod:          adxPeriod,
		StopLossMultiplier: stopLossMultiplier,
		RiskAmount:         riskAmount,
		LarkTimeout:        larkTimeout,
		Socks5Proxy:    getEnv("SOCKS5_PROXY", ""),
		BullishBreakoutWeight: bullishBreakoutWeight,
		BearishMomentumWeight: bearishMomentumWeight,
		PossibleFakeoutWeight: possibleFakeoutWeight,
		MarketContractionWeight: marketContractionWeight,
		HypeSymbol:            getEnv("HYPE_SYMBOL", "HYPEUSDT"),
		HypeOIStopThreshold:   hypeOIStopThreshold,
		HypeFRExtremeThreshold: hypeFRExtremeThreshold,
		HypeFRRecoveryThreshold: hypeFRRecoveryThreshold,
		HypeHigherLowPct:      hypeHigherLowPct,
		HypeSqueezePricePct:   hypeSqueezePricePct,
		HypeSqueezeOIDeclinePct: hypeSqueezeOIDeclinePct,
		HypeCooldownMinutes:   hypeCooldownMinutes,
		HypeLookbackKlines:    hypeLookbackKlines,
		HypeFundingInterval:   hypeFundingInterval,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
