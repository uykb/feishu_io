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
}

func Load() *Config {
	// 尝试加载 .env 文件（Docker中不需要）
	if err := godotenv.Load(); err != nil {
		log.Println("Info: .env file not found, using environment variables (normal for Docker)")
	}

	oiThreshold, _ := strconv.ParseFloat(getEnv("OI_THRESHOLD", "5.0"), 64)
	priceThreshold, _ := strconv.ParseFloat(getEnv("PRICE_THRESHOLD", "2.0"), 64)
	checkInterval, _ := strconv.Atoi(getEnv("CHECK_INTERVAL", "60"))

	return &Config{
		LarkWebhookURL: getEnv("LARK_WEBHOOK_URL", ""),
		OIThreshold:    oiThreshold,
		PriceThreshold: priceThreshold,
		CheckInterval:  checkInterval,
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
