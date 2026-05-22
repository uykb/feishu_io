package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"binance-monitor/binance"
	"binance-monitor/config"
	"binance-monitor/lark"
	"binance-monitor/models"
	"binance-monitor/strategy"
)

func main() {
	log.Println("=== 币安加密货币市场监控程序启动 ===")

	cfg := config.Load()
	if cfg.LarkWebhookURL == "" {
		log.Fatal("错误: 请配置 LARK_WEBHOOK_URL 环境变量")
	}

	if cfg.HypeOnlyMode {
		runHypeOnlyMode(cfg)
	} else {
		runFullMode(cfg)
	}
}

func runFullMode(cfg *config.Config) {
	log.Printf("模式: 全量监控 | OI阈值=%.1f%%, 价格阈值=%.1f%%, 检查间隔=%ds",
		cfg.OIThreshold, cfg.PriceThreshold, cfg.CheckInterval)

	klineDataCh := make(chan models.KlineData, 1000)
	oiDataCh := make(chan models.OIData, 1000)
	signalCh := make(chan models.Signal, 100)

	oiFetcher := binance.NewOIFetcher(oiDataCh, time.Duration(cfg.MinCheckInterval)*time.Second, time.Duration(cfg.CheckInterval)*time.Second, cfg.Socks5Proxy)
	log.Println("正在获取USDT永续合约交易对列表...")
	symbols, err := oiFetcher.FetchSymbols()
	if err != nil {
		log.Fatalf("获取交易对失败: %v", err)
	}

	klineSubscriber := binance.NewKlineSubscriber(symbols, klineDataCh, cfg.Socks5Proxy)
	if err := klineSubscriber.Start(); err != nil {
		log.Fatalf("启动WebSocket订阅失败: %v", err)
	}
	defer klineSubscriber.Close()

	detectorConfig := strategy.DetectorConfig{
		OIThreshold:             cfg.OIThreshold,
		PriceThreshold:          cfg.PriceThreshold,
		ADXThreshold:            cfg.ADXThreshold,
		ATRPeriod:               cfg.ATRPeriod,
		ADXPeriod:               cfg.ADXPeriod,
		StopLossMultiplier:      cfg.StopLossMultiplier,
		RiskAmount:              cfg.RiskAmount,
		BullishBreakoutWeight:   cfg.BullishBreakoutWeight,
		BearishMomentumWeight:   cfg.BearishMomentumWeight,
		PossibleFakeoutWeight:   cfg.PossibleFakeoutWeight,
		MarketContractionWeight: cfg.MarketContractionWeight,
	}
	detector := strategy.NewSignalDetector(detectorConfig, signalCh)

	bot := lark.NewBot(cfg.LarkWebhookURL, time.Duration(cfg.LarkTimeout)*time.Second)

	hypeSignalCh := make(chan models.HypeSignal, 50)
	fundingCh := make(chan models.FundingRateData, 100)

	hypeDetectorConfig := strategy.HypeDetectorConfig{
		OIStopThreshold:      cfg.HypeOIStopThreshold,
		FRExtremeThreshold:   cfg.HypeFRExtremeThreshold,
		FRRecoveryThreshold:  cfg.HypeFRRecoveryThreshold,
		HigherLowPct:         cfg.HypeHigherLowPct,
		SqueezePricePct:      cfg.HypeSqueezePricePct,
		SqueezeOIDeclinePct:  cfg.HypeSqueezeOIDeclinePct,
		CooldownMinutes:      cfg.HypeCooldownMinutes,
		LookbackKlines:       cfg.HypeLookbackKlines,
		ADXThreshold:         cfg.ADXThreshold,
		ADXPeriod:            cfg.ADXPeriod,
		ATRPeriod:            cfg.ATRPeriod,
	}
	hypeDetector := strategy.NewHypeDetector(cfg.HypeSymbol, hypeDetectorConfig, hypeSignalCh)
	fundingFetcher := binance.NewFundingFetcher(cfg.HypeSymbol, fundingCh, time.Duration(cfg.HypeFundingInterval)*time.Second, cfg.Socks5Proxy)

	log.Println("启动数据处理协程...")

	go detector.ProcessKlineData(klineDataCh)
	log.Println("✓ K线数据处理协程已启动")

	go detector.ProcessOIData(oiDataCh)
	log.Println("✓ OI数据处理协程已启动")

	go oiFetcher.Start()
	log.Println("✓ OI数据获取协程已启动")

	go bot.ProcessSignals(signalCh)
	log.Println("✓ 飞书消息发送协程已启动")

	go hypeDetector.ProcessKlineData(klineDataCh)
	log.Printf("✓ HYPE %s 检测协程已启动", cfg.HypeSymbol)

	go hypeDetector.ProcessOIData(oiDataCh)
	log.Println("✓ HYPE OI处理协程已启动")

	go fundingFetcher.Start()
	log.Printf("✓ HYPE 资金费率轮询协程已启动 (间隔: %ds)", cfg.HypeFundingInterval)

	go hypeDetector.ProcessFundingRate(fundingCh)
	log.Println("✓ HYPE 资金费率处理协程已启动")

	go bot.ProcessHypeSignals(hypeSignalCh)
	log.Println("✓ HYPE 消息发送协程已启动")

	log.Println("=== 所有模块启动成功，开始监控... ===")
	log.Printf("监控 %d 个交易对的15分钟K线和持仓量变化", len(symbols))
	log.Printf("HYPE专属策略: %s", cfg.HypeSymbol)

	waitForExit(klineSubscriber, klineDataCh, oiDataCh, signalCh, hypeSignalCh, fundingCh)
}

func runHypeOnlyMode(cfg *config.Config) {
	log.Printf("模式: HYPE专属 | 交易对: %s", cfg.HypeSymbol)

	klineDataCh := make(chan models.KlineData, 100)
	oiDataCh := make(chan models.OIData, 100)
	hypeSignalCh := make(chan models.HypeSignal, 50)
	fundingCh := make(chan models.FundingRateData, 100)

	hypeSymbols := []string{cfg.HypeSymbol}

	klineSubscriber := binance.NewKlineSubscriber(hypeSymbols, klineDataCh, cfg.Socks5Proxy)
	if err := klineSubscriber.Start(); err != nil {
		log.Fatalf("启动WebSocket订阅失败: %v", err)
	}
	defer klineSubscriber.Close()

	oiFetcher := binance.NewOIFetcher(oiDataCh, time.Duration(cfg.MinCheckInterval)*time.Second, time.Duration(cfg.CheckInterval)*time.Second, cfg.Socks5Proxy)
	oiFetcher.SetSymbols(hypeSymbols)

	hypeDetectorConfig := strategy.HypeDetectorConfig{
		OIStopThreshold:      cfg.HypeOIStopThreshold,
		FRExtremeThreshold:   cfg.HypeFRExtremeThreshold,
		FRRecoveryThreshold:  cfg.HypeFRRecoveryThreshold,
		HigherLowPct:         cfg.HypeHigherLowPct,
		SqueezePricePct:      cfg.HypeSqueezePricePct,
		SqueezeOIDeclinePct:  cfg.HypeSqueezeOIDeclinePct,
		CooldownMinutes:      cfg.HypeCooldownMinutes,
		LookbackKlines:       cfg.HypeLookbackKlines,
		ADXThreshold:         cfg.ADXThreshold,
		ADXPeriod:            cfg.ADXPeriod,
		ATRPeriod:            cfg.ATRPeriod,
	}
	hypeDetector := strategy.NewHypeDetector(cfg.HypeSymbol, hypeDetectorConfig, hypeSignalCh)
	fundingFetcher := binance.NewFundingFetcher(cfg.HypeSymbol, fundingCh, time.Duration(cfg.HypeFundingInterval)*time.Second, cfg.Socks5Proxy)
	bot := lark.NewBot(cfg.LarkWebhookURL, time.Duration(cfg.LarkTimeout)*time.Second)

	log.Println("启动数据处理协程...")

	go hypeDetector.ProcessKlineData(klineDataCh)
	log.Printf("✓ HYPE %s K线检测协程已启动", cfg.HypeSymbol)

	go hypeDetector.ProcessOIData(oiDataCh)
	log.Printf("✓ HYPE %s OI处理协程已启动", cfg.HypeSymbol)

	go oiFetcher.Start()
	log.Printf("✓ HYPE %s OI轮询协程已启动 (间隔: %ds)", cfg.HypeSymbol, cfg.MinCheckInterval)

	go fundingFetcher.Start()
	log.Printf("✓ HYPE %s 资金费率轮询协程已启动 (间隔: %ds)", cfg.HypeSymbol, cfg.HypeFundingInterval)

	go hypeDetector.ProcessFundingRate(fundingCh)
	log.Println("✓ HYPE 资金费率处理协程已启动")

	go bot.ProcessHypeSignals(hypeSignalCh)
	log.Println("✓ HYPE 消息发送协程已启动")

	log.Println("=== 所有模块启动成功，开始监控... ===")
	log.Printf("监控交易对: %s", cfg.HypeSymbol)

	waitForExit(klineSubscriber, klineDataCh, oiDataCh, nil, hypeSignalCh, fundingCh)
}

func waitForExit(
	klineSubscriber *binance.KlineSubscriber,
	klineDataCh chan models.KlineData,
	oiDataCh chan models.OIData,
	signalCh chan models.Signal,
	hypeSignalCh chan models.HypeSignal,
	fundingCh chan models.FundingRateData,
) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n=== 收到退出信号，正在关闭程序... ===")
	klineSubscriber.Close()
	close(klineDataCh)
	close(oiDataCh)
	if signalCh != nil {
		close(signalCh)
	}
	if hypeSignalCh != nil {
		close(hypeSignalCh)
	}
	close(fundingCh)
	log.Println("程序已安全退出")
}
