package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	client      *http.Client
	symbols     []string
	oiDataCh    chan models.OIData
	interval    time.Duration
	concurrency int
	mu          sync.RWMutex
}

// NewOIFetcher 创建持仓量获取器
func NewOIFetcher(oiDataCh chan models.OIData, interval time.Duration) *OIFetcher {
	return &OIFetcher{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		oiDataCh:    oiDataCh,
		interval:    interval,
		concurrency: 20, // 并发协程数
		mu:          sync.RWMutex{},
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
	oiTicker := time.NewTicker(of.interval)
	defer oiTicker.Stop()

	// 每小时更新一次交易对列表
	symbolUpdateTicker := time.NewTicker(1 * time.Hour)
	defer symbolUpdateTicker.Stop()

	// 立即执行一次
	of.fetchAllOI()

	for {
		select {
		case <-oiTicker.C:
			of.fetchAllOI()
		case <-symbolUpdateTicker.C:
			log.Println("定时任务: 正在更新交易对列表...")
			if _, err := of.FetchSymbols(); err != nil {
				log.Printf("定时更新交易对列表失败: %v", err)
			}
		}
	}
}

// fetchAllOI 并发获取所有交易对的持仓量
func (of *OIFetcher) fetchAllOI() {
	of.mu.RLock()
	symbolsToFetch := make([]string, len(of.symbols))
	copy(symbolsToFetch, of.symbols)
	of.mu.RUnlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, of.concurrency)

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

			select {
			case of.oiDataCh <- oiData:
			default:
				log.Printf("OI数据通道已满，丢弃 %s 的数据", sym)
			}
		}(symbol)
	}

	wg.Wait()
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
	// 返回一个副本以防止外部修改
	symbolsCopy := make([]string, len(of.symbols))
	copy(symbolsCopy, of.symbols)
	return symbolsCopy
}
