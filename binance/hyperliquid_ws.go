package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"binance-monitor/models"

	"github.com/gorilla/websocket"
)

const (
	hyperliquidWSURL = "wss://api.hyperliquid.xyz/ws"
)

// HLCandle Hyperliquid蜡烛数据
type HLCandle struct {
	T int64  `json:"t"`
	Tt int64  `json:"T"`
	S string `json:"s"`
	I string `json:"i"`
	O string `json:"o"`
	C string `json:"c"`
	H string `json:"h"`
	L string `json:"l"`
	V string `json:"v"`
	N int    `json:"n"`
}

// HLWSMessage Hyperliquid WebSocket消息
type HLWSMessage struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

// HLSubscriptionResponse Hyperliquid订阅响应
type HLSubscriptionResponse struct {
	Method string `json:"method"`
}

// HyperliquidSubscriber Hyperliquid WebSocket订阅器
type HyperliquidSubscriber struct {
	symbols     []string
	interval    string
	klineDataCh chan models.KlineData
	conn        *websocket.Conn
	mu          sync.Mutex
	proxyURL    string
}

// NewHyperliquidSubscriber 创建Hyperliquid订阅器
func NewHyperliquidSubscriber(symbols []string, klineDataCh chan models.KlineData, interval string, proxyURL string) *HyperliquidSubscriber {
	if interval == "" {
		interval = "15m"
	}
	return &HyperliquidSubscriber{
		symbols:     symbols,
		interval:    interval,
		klineDataCh: klineDataCh,
		proxyURL:    proxyURL,
	}
}

// Start 启动WebSocket订阅
func (hs *HyperliquidSubscriber) Start() error {
	if err := hs.connect(); err != nil {
		return err
	}

	go hs.readMessages()
	go hs.heartbeat()

	return nil
}

// connect 建立连接
func (hs *HyperliquidSubscriber) connect() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	dialer := websocket.DefaultDialer
	if hs.proxyURL != "" {
		parsedURL, err := url.Parse(hs.proxyURL)
		if err != nil {
			log.Printf("Warning: Invalid proxy URL: %v", err)
		} else {
			dialer = &websocket.Dialer{
				Proxy:            http.ProxyURL(parsedURL),
				HandshakeTimeout: 45 * time.Second,
			}
			log.Printf("Enabled proxy for Hyperliquid WebSocket: %s", hs.proxyURL)
		}
	}

	conn, _, err := dialer.Dial(hyperliquidWSURL, nil)
	if err != nil {
		return fmt.Errorf("Hyperliquid WebSocket连接失败: %w", err)
	}

	hs.conn = conn

	if err := hs.subscribeAll(); err != nil {
		conn.Close()
		return err
	}

	log.Printf("Hyperliquid WebSocket已连接，订阅 %d 个交易对的%s K线", len(hs.symbols), hs.interval)
	return nil
}

// subscribeAll 订阅所有交易对
func (hs *HyperliquidSubscriber) subscribeAll() error {
	for _, symbol := range hs.symbols {
		coin := models.SourceHyperliquid.NormalizeSymbol(symbol)

		req := map[string]interface{}{
			"method": "subscribe",
			"subscription": map[string]interface{}{
				"type":     "candle",
				"coin":     coin,
				"interval": hs.interval,
			},
		}

		if err := hs.conn.WriteJSON(req); err != nil {
			return fmt.Errorf("订阅 %s 失败: %w", symbol, err)
		}

		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

// readMessages 读取消息
func (hs *HyperliquidSubscriber) readMessages() {
	for {
		_, message, err := hs.conn.ReadMessage()
		if err != nil {
			log.Printf("Hyperliquid WebSocket读取错误: %v", err)
			hs.reconnect()
			return
		}

		var msg HLWSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Channel {
		case "candle":
			hs.handleCandle(msg.Data)
		case "subscriptionResponse":
			// 订阅确认，忽略
		}
	}
}

// handleCandle 处理K线数据
func (hs *HyperliquidSubscriber) handleCandle(data json.RawMessage) {
	var candles []HLCandle
	if err := json.Unmarshal(data, &candles); err != nil {
		return
	}

	for _, c := range candles {
		open, _ := strconv.ParseFloat(c.O, 64)
		close, _ := strconv.ParseFloat(c.C, 64)
		high, _ := strconv.ParseFloat(c.H, 64)
		low, _ := strconv.ParseFloat(c.L, 64)

		displaySymbol := models.SourceHyperliquid.DisplaySymbol(c.S)

		klineData := models.KlineData{
			Symbol:    displaySymbol,
			Source:    models.SourceHyperliquid,
			Timestamp: time.Unix(c.T/1000, 0),
			Open:      open,
			Close:     close,
			High:      high,
			Low:       low,
		}

		select {
		case hs.klineDataCh <- klineData:
		default:
			log.Printf("Hyperliquid K线通道已满，丢弃 %s 的数据", klineData.Symbol)
		}
	}
}

// heartbeat 心跳
func (hs *HyperliquidSubscriber) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		hs.mu.Lock()
		if hs.conn != nil {
			if err := hs.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Hyperliquid心跳发送失败: %v", err)
				hs.mu.Unlock()
				hs.reconnect()
				return
			}
		}
		hs.mu.Unlock()
	}
}

// reconnect 重连
func (hs *HyperliquidSubscriber) reconnect() {
	hs.mu.Lock()
	if hs.conn != nil {
		hs.conn.Close()
	}
	hs.mu.Unlock()

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	log.Println("Hyperliquid WebSocket连接断开，准备重连...")

	for {
		log.Printf("Hyperliquid尝试重连... (等待 %v)", backoff)
		time.Sleep(backoff)

		if err := hs.connect(); err != nil {
			log.Printf("Hyperliquid重连失败: %v", err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		go hs.readMessages()
		log.Println("Hyperliquid WebSocket重连成功")
		return
	}
}

// Close 关闭连接
func (hs *HyperliquidSubscriber) Close() {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if hs.conn != nil {
		hs.conn.Close()
	}
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
