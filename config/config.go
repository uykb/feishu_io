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
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
