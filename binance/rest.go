package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"binance-monitor/models"
)

const (
	restBaseURL = "https://fapi.binance.com"
)

// ExchangeInfo 交易所信息响应
type ExchangeInfoResponse struct {
	Symbols []struct {
		Symbol             string `json:"symbol"`
		Status             string `json:"status"`
		ContractType       string `json:"contractType"`
		UnderlyingType     string `json:"underlyingType"`
		QuoteAsset         string `json:"quoteAsset"`
	} `json:"symbols"`
}

// OpenInterestResponse 持仓量响应
type OpenInterestResponse struct {
	Symbol           string `json:"symbol"`
	OpenInterest     string `json:"openInterest"`
	Time             int64  `json:"time"`
}

// OIFetcher 持仓量获取器
type OIFetcher struct {
	client          *http.Client
	symbols         []string
	oiDataCh        chan models.OIData
	minInterval     time.Duration
	maxInterval     time.Duration
	currentInterval time.Duration
	concurrency     int
	mu              sync.RWMutex
	lastOIData      map[string]models.OIData
}

// NewOIFetcher 创建持仓量获取器
func NewOIFetcher(oiDataCh chan models.OIData, minInterval, maxInterval time.Duration, proxyURL string) *OIFetcher {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxIdleConnsPerHost = 50
	t.IdleConnTimeout = 90 * time.Second

	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("Warning: Invalid SOCKS5 proxy URL: %v", err)
		} else {
			t.Proxy = http.ProxyURL(parsedURL)
			log.Printf("Enabled proxy for REST API: %s", proxyURL)
		}
	}

	return &OIFetcher{
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: t,
		},
		oiDataCh:        oiDataCh,
		minInterval:     minInterval,
		maxInterval:     maxInterval,
		currentInterval: maxInterval,
		concurrency:     20, // 并发协程数
		mu:              sync.RWMutex{},
		lastOIData:      make(map[string]models.OIData),
	}
}

// FetchSymbols 获取所有USDT永续合约交易对
func (of *OIFetcher) FetchSymbols() ([]string, error) {
	url := fmt.Sprintf("%s/fapi/v1/exchangeInfo", restBaseURL)
	
	resp, err := of.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取交易对列表失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var exchangeInfo ExchangeInfoResponse
	if err := json.Unmarshal(body, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	var symbols []string
	validSymbol := regexp.MustCompile(`^[A-Z0-9]+$`)
	for _, s := range exchangeInfo.Symbols {
		// 只选择USDT永续合约且状态为TRADING的交易对
		if s.Status == "TRADING" && s.ContractType == "PERPETUAL" && s.QuoteAsset == "USDT" && validSymbol.MatchString(s.Symbol) {
			symbols = append(symbols, s.Symbol)
		}
	}

	of.mu.Lock()
	of.symbols = symbols
	of.mu.Unlock()

	log.Printf("更新交易对列表: 获取到 %d 个USDT永续合约交易对", len(symbols))
	return symbols, nil
}

// Start 启动持仓量轮询
func (of *OIFetcher) Start() {
	// 每小时更新一次交易对列表
	symbolUpdateTicker := time.NewTicker(1 * time.Hour)
	defer symbolUpdateTicker.Stop()

	// 立即执行一次
	of.fetchAllOI()

	// 初始定时器
	timer := time.NewTimer(of.currentInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// 获取当前周期的最大波动率
			maxVolatility := of.fetchAllOI()

			// 根据波动率调整下一次的轮询间隔
			of.adjustInterval(maxVolatility)
			
			// 重置定时器
			timer.Reset(of.currentInterval)
			// log.Printf("下次轮询间隔: %v (最大波动率: %.4f%%)", of.currentInterval, maxVolatility)

		case <-symbolUpdateTicker.C:
			log.Println("定时任务: 正在更新交易对列表...")
			if _, err := of.FetchSymbols(); err != nil {
				log.Printf("定时更新交易对列表失败: %v", err)
			}
		}
	}
}

// adjustInterval 根据波动率调整轮询间隔
func (of *OIFetcher) adjustInterval(volatility float64) {
	// 如果波动率超过0.5%，使用最小间隔
	if volatility > 0.5 {
		of.currentInterval = of.minInterval
		return
	}

	// 否则线性增加间隔
	// volatility 0% -> maxInterval
	// volatility 0.5% -> minInterval
	ratio := volatility / 0.5
	if ratio > 1 {
		ratio = 1
	}

	// 线性插值
	intervalDiff := float64(of.maxInterval - of.minInterval)
	newInterval := float64(of.maxInterval) - (intervalDiff * ratio)
	
	of.currentInterval = time.Duration(newInterval)
	if of.currentInterval < of.minInterval {
		of.currentInterval = of.minInterval
	}
	if of.currentInterval > of.maxInterval {
		of.currentInterval = of.maxInterval
	}
}

// fetchAllOI 并发获取所有交易对的持仓量，返回最大波动率
func (of *OIFetcher) fetchAllOI() float64 {
	of.mu.RLock()
	symbolsToFetch := make([]string, len(of.symbols))
	copy(symbolsToFetch, of.symbols)
	of.mu.RUnlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, of.concurrency)
	
	// 用于收集每个交易对的波动率
	volatilityCh := make(chan float64, len(symbolsToFetch))

	for _, symbol := range symbolsToFetch {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			oiData, err := of.fetchOI(sym)
			if err != nil {
				log.Printf("获取 %s 持仓量失败: %v", sym, err)
				return
			}

			// 数据去重与波动率计算
			of.mu.Lock()
			lastData, exists := of.lastOIData[sym]
			of.lastOIData[sym] = oiData
			of.mu.Unlock()

			var changePercent float64
			if exists && lastData.OpenInterest > 0 {
				changePercent = math.Abs((oiData.OpenInterest - lastData.OpenInterest) / lastData.OpenInterest * 100)
				
				// 只有当数据有变化或者是新数据时才发送
				if changePercent > 0.0001 || oiData.Timestamp.After(lastData.Timestamp) {
					select {
					case of.oiDataCh <- oiData:
					default:
						// log.Printf("OI数据通道已满，丢弃 %s 的数据", sym)
					}
				}
			} else {
				// 第一次获取，直接发送
				select {
				case of.oiDataCh <- oiData:
				default:
				}
			}

			volatilityCh <- changePercent

		}(symbol)

		// 速率限制: 35ms延迟 -> ~28 req/s -> ~1680 req/min
		// Binance IP限制为 2400 req/min，留出缓冲空间
		time.Sleep(35 * time.Millisecond)
	}

	wg.Wait()
	close(volatilityCh)

	// 计算最大波动率
	var maxVolatility float64
	for v := range volatilityCh {
		if v > maxVolatility {
			maxVolatility = v
		}
	}
	
	return maxVolatility
}

// fetchOI 获取单个交易对的持仓量
func (of *OIFetcher) fetchOI(symbol string) (models.OIData, error) {
	url := fmt.Sprintf("%s/fapi/v1/openInterest?symbol=%s", restBaseURL, symbol)
	
	resp, err := of.client.Get(url)
	if err != nil {
		return models.OIData{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.OIData{}, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return models.OIData{}, fmt.Errorf("HTTP错误: %d, URL: %s, Body: %s", resp.StatusCode, url, string(body))
	}
	if err != nil {
		return models.OIData{}, err
	}

	var oiResp OpenInterestResponse
	if err := json.Unmarshal(body, &oiResp); err != nil {
		return models.OIData{}, fmt.Errorf("解析JSON失败: %w, Body: %s", err, string(body))
	}

	oi, err := strconv.ParseFloat(oiResp.OpenInterest, 64)
	if err != nil {
		return models.OIData{}, err
	}

	return models.OIData{
		Symbol:       oiResp.Symbol,
		OpenInterest: oi,
		Timestamp:    time.Unix(oiResp.Time/1000, 0),
	}, nil
}

// GetSymbols 返回交易对列表
func (of *OIFetcher) GetSymbols() []string {
	of.mu.RLock()
	defer of.mu.RUnlock()
	symbolsCopy := make([]string, len(of.symbols))
	copy(symbolsCopy, of.symbols)
	return symbolsCopy
}

// SetSymbols 设置交易对列表（用于指定监控特定交易对）
func (of *OIFetcher) SetSymbols(symbols []string) {
	of.mu.Lock()
	defer of.mu.Unlock()
	of.symbols = symbols
	log.Printf("手动设置交易对列表: %d 个交易对 %v", len(symbols), symbols)
}
