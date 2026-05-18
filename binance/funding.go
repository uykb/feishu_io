package binance

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"binance-monitor/models"
)

// FundingRateResponse 资金费率响应
type FundingRateResponse struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
}

// FundingFetcher 资金费率获取器
type FundingFetcher struct {
	client    *http.Client
	symbol    string
	fundingCh chan models.FundingRateData
	interval  time.Duration
}

// NewFundingFetcher 创建资金费率获取器
func NewFundingFetcher(symbol string, fundingCh chan models.FundingRateData, interval time.Duration, proxyURL string) *FundingFetcher {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 10
	t.IdleConnTimeout = 30 * time.Second

	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("Warning: Invalid SOCKS5 proxy URL: %v", err)
		} else {
			t.Proxy = http.ProxyURL(parsedURL)
			log.Printf("Enabled proxy for Funding API: %s", proxyURL)
		}
	}

	return &FundingFetcher{
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: t,
		},
		symbol:    symbol,
		fundingCh: fundingCh,
		interval:  interval,
	}
}

// Start 启动资金费率轮询
func (ff *FundingFetcher) Start() {
	log.Printf("开始轮询 %s 资金费率，间隔: %v", ff.symbol, ff.interval)

	// 立即执行一次
	ff.fetchFundingRate()

	ticker := time.NewTicker(ff.interval)
	defer ticker.Stop()

	for range ticker.C {
		ff.fetchFundingRate()
	}
}

// fetchFundingRate 获取资金费率
func (ff *FundingFetcher) fetchFundingRate() {
	url := fmt.Sprintf("%s/fapi/v1/fundingRate?symbol=%s&limit=1", restBaseURL, ff.symbol)

	resp, err := ff.client.Get(url)
	if err != nil {
		log.Printf("获取 %s 资金费率失败: %v", ff.symbol, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取 %s 资金费率响应失败: %v", ff.symbol, err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("获取 %s 资金费率 HTTP错误: %d, Body: %s", ff.symbol, resp.StatusCode, string(body))
		return
	}

	var results []FundingRateResponse
	if err := json.Unmarshal(body, &results); err != nil {
		log.Printf("解析 %s 资金费率JSON失败: %v, Body: %s", ff.symbol, err, string(body))
		return
	}

	if len(results) == 0 {
		return
	}

	r := results[0]
	fundingRate, err := strconv.ParseFloat(r.FundingRate, 64)
	if err != nil {
		log.Printf("解析 %s 资金费率数值失败: %v", ff.symbol, err)
		return
	}

	data := models.FundingRateData{
		Symbol:      r.Symbol,
		FundingRate: fundingRate,
		FundingTime: time.Unix(r.FundingTime/1000, 0),
		Timestamp:   time.Now(),
	}

	select {
	case ff.fundingCh <- data:
	default:
		// log.Printf("资金费率通道已满，丢弃 %s 的数据", ff.symbol)
	}
}
