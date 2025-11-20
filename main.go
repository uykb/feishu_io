package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"binance-monitor/binance"
	"binance-monitor/config"
	"binance-monitor/models"
	"binance-monitor/strategy"
	"binance-monitor/telegram"
)

func main() {
	log.Println("=== 币安加密货币市场监控程序启动 ===")

	// 加载配置
	cfg := config.Load()
	if cfg.LarkWebhookURL == "" {
		log.Fatal("错误: 请配置 LARK_WEBHOOK_URL 环境变量")
	}

	log.Printf("配置加载成功: OI阈值=%.1f%%, 价格阈值=%.1f%%, 检查间隔=%ds",
		cfg.OIThreshold, cfg.PriceThreshold, cfg.CheckInterval)

	// 创建通道
	klineDataCh := make(chan models.KlineData, 1000)
	oiDataCh := make(chan models.OIData, 1000)
	signalCh := make(chan models.Signal, 100)

	// 初始化OI获取器并获取交易对列表
	oiFetcher := binance.NewOIFetcher(oiDataCh, time.Duration(cfg.CheckInterval)*time.Second)
	log.Println("正在获取USDT永续合约交易对列表...")
	symbols, err := oiFetcher.FetchSymbols()
	if err != nil {
		log.Fatalf("获取交易对失败: %v", err)
	}

	// 初始化WebSocket订阅器
	klineSubscriber := binance.NewKlineSubscriber(symbols, klineDataCh)
	if err := klineSubscriber.Start(); err != nil {
		log.Fatalf("启动WebSocket订阅失败: %v", err)
	}
	defer klineSubscriber.Close()

	// 初始化信号检测器
	detector := strategy.NewSignalDetector(cfg.OIThreshold, cfg.PriceThreshold, signalCh)

	// 初始化飞书机器人
	bot := lark.NewBot(cfg.LarkWebhookURL)

	// 启动各个协程
	log.Println("启动数据处理协程...")

	// 1. K线数据处理协程
	go detector.ProcessKlineData(klineDataCh)
	log.Println("✓ K线数据处理协程已启动")

	// 2. OI数据处理协程
	go detector.ProcessOIData(oiDataCh)
	log.Println("✓ OI数据处理协程已启动")

	// 3. OI数据获取协程
	go oiFetcher.Start()
	log.Println("✓ OI数据获取协程已启动")

	// 4. 信号发送协程
	go bot.ProcessSignals(signalCh)
	log.Println("✓ 飞书消息发送协程已启动")

	log.Println("=== 所有模块启动成功，开始监控... ===")
	log.Printf("监控 %d 个交易对的15分钟K线和持仓量变化", len(symbols))

	// 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n=== 收到退出信号，正在关闭程序... ===")
	klineSubscriber.Close()
	close(klineDataCh)
	close(oiDataCh)
	close(signalCh)
	log.Println("程序已安全退出")
}
