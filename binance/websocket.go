package binance

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
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
}

// NewKlineSubscriber 创建K线订阅器
func NewKlineSubscriber(symbols []string, klineDataCh chan models.KlineData) *KlineSubscriber {
	return &KlineSubscriber{
		symbols:     symbols,
		klineDataCh: klineDataCh,
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

	// 构建订阅流
	streams := make([]string, len(ks.symbols))
	for i, symbol := range ks.symbols {
		streams[i] = fmt.Sprintf("%s@kline_15m", strings.ToLower(symbol))
	}
	streamParam := strings.Join(streams, "/")
	url := fmt.Sprintf("%s?streams=%s", wsURL, streamParam)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %w", err)
	}

	ks.conn = conn
	log.Printf("WebSocket已连接，订阅 %d 个交易对的15分钟K线", len(ks.symbols))
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

	log.Println("WebSocket连接断开，5秒后尝试重连...")
	time.Sleep(5 * time.Second)

	for {
		if err := ks.connect(); err != nil {
			log.Printf("重连失败: %v，10秒后重试", err)
			time.Sleep(10 * time.Second)
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
