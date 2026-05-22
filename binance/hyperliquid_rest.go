package binance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"binance-monitor/models"
)

const (
	hyperliquidInfoURL = "https://api.hyperliquid.xyz/info"
)

// HLMetaResponse Hyperliquid meta response
type HLMetaResponse struct {
	Universe []struct {
		Name         string `json:"name"`
		SzDecimals   int    `json:"szDecimals"`
		MaxLeverage  int    `json:"maxLeverage"`
	} `json:"universe"`
}

// HLAssetCtx Hyperliquid asset context
type HLAssetCtx struct {
	OpenInterest string `json:"openInterest"`
	MarkPx       string `json:"markPx"`
	Funding      string `json:"funding"`
}

// HLFundingHistory Hyperliquid funding history item
type HLFundingHistory struct {
	Coin        string `json:"coin"`
	FundingRate string `json:"fundingRate"`
	Premium     string `json:"premium"`
	Time        int64  `json:"time"`
}

// HyperliquidFetcher Hyperliquid OI and Funding fetcher
type HyperliquidFetcher struct {
	client    *http.Client
	symbols   []string
	oiDataCh  chan models.OIData
	fundingCh chan models.FundingRateData
	interval  time.Duration
	mu        sync.RWMutex
	lastOI    map[string]float64
	lastFR    map[string]float64
}

// NewHyperliquidFetcher 创建Hyperliquid数据获取器
func NewHyperliquidFetcher(symbols []string, oiDataCh chan models.OIData, fundingCh chan models.FundingRateData, interval time.Duration, proxyURL string) *HyperliquidFetcher {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 10
	t.IdleConnTimeout = 30 * time.Second

	if proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			log.Printf("Warning: Invalid proxy URL: %v", err)
		} else {
			t.Proxy = http.ProxyURL(parsedURL)
			log.Printf("Enabled proxy for Hyperliquid API: %s", proxyURL)
		}
	}

	lastOI := make(map[string]float64)
	lastFR := make(map[string]float64)
	for _, s := range symbols {
		lastOI[s] = 0
		lastFR[s] = 0
	}

	return &HyperliquidFetcher{
		client: &http.Client{
			Timeout:   15 * time.Second,
			Transport: t,
		},
		symbols:   symbols,
		oiDataCh:  oiDataCh,
		fundingCh: fundingCh,
		interval:  interval,
		lastOI:    lastOI,
		lastFR:    lastFR,
	}
}

// FetchSymbols 获取Hyperliquid所有永续合约交易对
func (hf *HyperliquidFetcher) FetchSymbols() ([]string, error) {
	body := map[string]interface{}{
		"type": "meta",
	}

	resp, err := hf.doPost(body)
	if err != nil {
		return nil, fmt.Errorf("获取Hyperliquid交易对失败: %w", err)
	}

	var meta HLMetaResponse
	if err := json.Unmarshal(resp, &meta); err != nil {
		return nil, fmt.Errorf("解析Hyperliquid meta失败: %w", err)
	}

	var symbols []string
	for _, u := range meta.Universe {
		symbols = append(symbols, u.Name)
	}

	log.Printf("更新Hyperliquid交易对列表: 获取到 %d 个永续合约", len(symbols))
	return symbols, nil
}

// Start 启动轮询
func (hf *HyperliquidFetcher) Start() {
	log.Printf("开始轮询Hyperliquid OI和资金费率，间隔: %v", hf.interval)

	hf.fetchAll()

	ticker := time.NewTicker(hf.interval)
	defer ticker.Stop()

	for range ticker.C {
		hf.fetchAll()
	}
}

// fetchAll 获取所有合约的OI和资金费率
func (hf *HyperliquidFetcher) fetchAll() {
	body := map[string]interface{}{
		"type": "metaAndAssetCtxs",
		"dex":  "",
	}

	resp, err := hf.doPost(body)
	if err != nil {
		log.Printf("获取Hyperliquid metaAndAssetCtxs失败: %v", err)
		return
	}

	// 响应是 [meta, assetCtxs] 的数组
	var rawResp []json.RawMessage
	if err := json.Unmarshal(resp, &rawResp); err != nil {
		log.Printf("解析Hyperliquid响应失败: %v", err)
		return
	}

	if len(rawResp) < 2 {
		return
	}

	var meta HLMetaResponse
	if err := json.Unmarshal(rawResp[0], &meta); err != nil {
		log.Printf("解析Hyperliquid meta失败: %v", err)
		return
	}

	var ctxs []HLAssetCtx
	if err := json.Unmarshal(rawResp[1], &ctxs); err != nil {
		log.Printf("解析Hyperliquid assetCtxs失败: %v", err)
		return
	}

	hf.mu.Lock()
	defer hf.mu.Unlock()

	for i, u := range meta.Universe {
		// 只获取我们关注的交易对
		if !hf.isWatching(u.Name) {
			continue
		}

		if i >= len(ctxs) {
			continue
		}

		ctx := ctxs[i]

		oi, err := strconv.ParseFloat(ctx.OpenInterest, 64)
		if err != nil {
			continue
		}

		funding, err := strconv.ParseFloat(ctx.Funding, 64)
		if err != nil {
			continue
		}

		markPx, _ := strconv.ParseFloat(ctx.MarkPx, 64)
		_ = markPx

		now := time.Now()
		displaySymbol := models.SourceHyperliquid.DisplaySymbol(u.Name)

		// OI变化检测
		prevOI := hf.lastOI[u.Name]
		if prevOI == 0 || oi != prevOI {
			hf.lastOI[u.Name] = oi
			select {
			case hf.oiDataCh <- models.OIData{
				Symbol:       displaySymbol,
				Source:       models.SourceHyperliquid,
				OpenInterest: oi,
				Timestamp:    now,
			}:
			default:
			}
		}

		// 资金费率变化检测
		prevFR := hf.lastFR[u.Name]
		if prevFR == 0 || funding != prevFR {
			hf.lastFR[u.Name] = funding
			select {
			case hf.fundingCh <- models.FundingRateData{
				Symbol:      displaySymbol,
				Source:      models.SourceHyperliquid,
				FundingRate: funding,
				FundingTime: now,
				Timestamp:   now,
			}:
			default:
			}
		}
	}
}

// isWatching 检查是否监控该交易对
func (hf *HyperliquidFetcher) isWatching(name string) bool {
	hf.mu.RLock()
	defer hf.mu.RUnlock()

	for _, s := range hf.symbols {
		normalized := models.SourceHyperliquid.NormalizeSymbol(s)
		if strings.EqualFold(normalized, name) {
			return true
		}
	}
	return false
}

// doPost 发送POST请求到Hyperliquid info端点
func (hf *HyperliquidFetcher) doPost(body interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	resp, err := hf.client.Post(hyperliquidInfoURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误: %d, Body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetSymbols 返回交易对列表
func (hf *HyperliquidFetcher) GetSymbols() []string {
	hf.mu.RLock()
	defer hf.mu.RUnlock()
	symbolsCopy := make([]string, len(hf.symbols))
	copy(symbolsCopy, hf.symbols)
	return symbolsCopy
}

// SetSymbols 设置交易对列表
func (hf *HyperliquidFetcher) SetSymbols(symbols []string) {
	hf.mu.Lock()
	defer hf.mu.Unlock()
	hf.symbols = symbols
	for _, s := range symbols {
		normalized := models.SourceHyperliquid.NormalizeSymbol(s)
		hf.lastOI[normalized] = 0
		hf.lastFR[normalized] = 0
	}
	log.Printf("Hyperliquid设置交易对列表: %v", symbols)
}
