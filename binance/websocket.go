package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"binance-monitor/models"

	"github.com/gorilla/websocket"
)

const (
	wsURL = "wss://fstream.binance.com/stream"
)

// KlineWSMessage WebSocket K线消息结构
type KlineWSMessage struct {
	Stream string `json:"stream"`
	Data   struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"`
		Symbol    string `json:"s"`
		Kline     struct {
			StartTime int64           `json:"t"`
			EndTime   int64           `json:"T"`
			Symbol    string          `json:"s"`
			Interval  string          `json:"i"`
			Open      json.Number     `json:"o"`
			Close     json.Number     `json:"c"`
			High      json.Number     `json:"h"`
			Low       json.Number     `json:"l"`
			IsClosed  bool            `json:"x"`
		} `json:"k"`
	} `json:"data"`
}

// KlineSubscriber WebSocket K线订阅器
type KlineSubscriber struct {
	symbols      []string
	klineDataCh  chan models.KlineData
	conn         *websocket.Conn
	mu           sync.Mutex
	reconnecting bool
	proxyURL     string
}

// NewKlineSubscriber 创建K线订阅器
func NewKlineSubscriber(symbols []string, klineDataCh chan models.KlineData, proxyURL string) *KlineSubscriber {
	return &KlineSubscriber{
		symbols:     symbols,
		klineDataCh: klineDataCh,
		proxyURL:    proxyURL,
	}
}

// Start 启动WebSocket订阅
func (ks *KlineSubscriber) Start() error {
	if err := ks.connect(); err != nil {
		return err
	}

	go ks.readMessages()
	go ks.heartbeat()

	return nil
}

// connect 建立WebSocket连接
func (ks *KlineSubscriber) connect() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	// 配置代理
	dialer := websocket.DefaultDialer
	if ks.proxyURL != "" {
		parsedURL, err := url.Parse(ks.proxyURL)
		if err != nil {
			log.Printf("Warning: Invalid SOCKS5 proxy URL: %v", err)
		} else {
			dialer = &websocket.Dialer{
				Proxy:            http.ProxyURL(parsedURL),
				HandshakeTimeout: 45 * time.Second,
			}
			log.Printf("Enabled proxy for WebSocket: %s", ks.proxyURL)
		}
	}

	// 直接连接到 stream 端点
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %w", err)
	}

	ks.conn = conn

	// 批量订阅
	if err := ks.subscribeAll(); err != nil {
		conn.Close()
		return err
	}

	log.Printf("WebSocket已连接，订阅 %d 个交易对的15分钟K线", len(ks.symbols))
	return nil
}

// subscribeAll 批量订阅所有交易对
func (ks *KlineSubscriber) subscribeAll() error {
	batchSize := 50
	for i := 0; i < len(ks.symbols); i += batchSize {
		end := i + batchSize
		if end > len(ks.symbols) {
			end = len(ks.symbols)
		}

		batch := ks.symbols[i:end]
		params := make([]string, len(batch))
		for j, symbol := range batch {
			params[j] = fmt.Sprintf("%s@kline_15m", strings.ToLower(symbol))
		}

		req := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": params,
			"id":     time.Now().UnixNano(),
		}

		if err := ks.conn.WriteJSON(req); err != nil {
			return fmt.Errorf("发送订阅请求失败: %w", err)
		}

		// 短暂延迟避免请求过快
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

// readMessages 读取WebSocket消息
func (ks *KlineSubscriber) readMessages() {
	for {
		_, message, err := ks.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket读取错误: %v", err)
			ks.reconnect()
			return
		}

		var wsMsg KlineWSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("解析消息失败: %v", err)
			continue
		}

		// 只处理已完成的K线
		if wsMsg.Data.Kline.IsClosed {
			klineData := ks.parseKlineData(&wsMsg)
			select {
			case ks.klineDataCh <- klineData:
			default:
				log.Printf("K线数据通道已满，丢弃 %s 的数据", klineData.Symbol)
			}
		}
	}
}

// parseKlineData 解析K线数据
func (ks *KlineSubscriber) parseKlineData(msg *KlineWSMessage) models.KlineData {
	open, _ := msg.Data.Kline.Open.Float64()
	close, _ := msg.Data.Kline.Close.Float64()
	high, _ := msg.Data.Kline.High.Float64()
	low, _ := msg.Data.Kline.Low.Float64()

	return models.KlineData{
		Symbol:    msg.Data.Symbol,
		Timestamp: time.Unix(msg.Data.Kline.StartTime/1000, 0),
		Open:      open,
		Close:     close,
		High:      high,
		Low:       low,
	}
}

// heartbeat 心跳检测
func (ks *KlineSubscriber) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ks.mu.Lock()
		if ks.conn != nil {
			if err := ks.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("心跳发送失败: %v", err)
				ks.mu.Unlock()
				ks.reconnect()
				return
			}
		}
		ks.mu.Unlock()
	}
}

// reconnect 重连逻辑
func (ks *KlineSubscriber) reconnect() {
	ks.mu.Lock()
	if ks.reconnecting {
		ks.mu.Unlock()
		return
	}
	ks.reconnecting = true
	if ks.conn != nil {
		ks.conn.Close()
	}
	ks.mu.Unlock()

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	log.Println("WebSocket连接断开，准备重连...")
	
	for {
		log.Printf("尝试重连... (等待 %v)", backoff)
		time.Sleep(backoff)

		if err := ks.connect(); err != nil {
			log.Printf("重连失败: %v", err)
			
			// 指数退避
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		ks.mu.Lock()
		ks.reconnecting = false
		ks.mu.Unlock()

		go ks.readMessages()
		log.Println("WebSocket重连成功")
		return
	}
}

// Close 关闭连接
func (ks *KlineSubscriber) Close() {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if ks.conn != nil {
		ks.conn.Close()
	}
}
