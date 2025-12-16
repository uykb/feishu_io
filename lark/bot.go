package lark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"binance-monitor/models"
)

// Bot 飞书机器人
type Bot struct {
	webhookURL string
	client     *http.Client
}

// NewBot 创建飞书机器人
func NewBot(webhookURL string, timeout time.Duration) *Bot {
	return &Bot{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// LarkCardMessage for interactive card
type LarkCardMessage struct {
	MsgType string   `json:"msg_type"`
	Card    LarkCard `json:"card"`
}

type LarkCard struct {
	Config   CardConfig    `json:"config"`
	Header   CardHeader    `json:"header"`
	Elements []interface{} `json:"elements"`
}

type CardConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type CardHeader struct {
	Title    CardText `json:"title"`
	Template string   `json:"template"`
	Extra    *CardText `json:"extra,omitempty"`
}

type CardText struct {
	Tag     string `json:"tag"`
	Content string `json:"content"`
}

type DivElement struct {
	Tag    string   `json:"tag"`
	Text   *CardText `json:"text,omitempty"`
	Fields []Field  `json:"fields,omitempty"`
}

type Field struct {
	IsShort bool     `json:"is_short"`
	Text    CardText `json:"text"`
}

type HrElement struct {
	Tag string `json:"tag"`
}

// SendSignal 发送交易信号
func (b *Bot) SendSignal(signal models.Signal) error {
	cardMessage := b.formatCardMessage(signal)
	jsonData, err := json.Marshal(cardMessage)
	if err != nil {
		return fmt.Errorf("序列化消息卡片失败: %w", err)
	}
	return b.sendMessage(jsonData)
}

// formatCardMessage 格式化消息为飞书卡片
func (b *Bot) formatCardMessage(signal models.Signal) LarkCardMessage {
	var signalDescription string
	var headerTemplate string

	switch signal.SignalType {
	case models.BullishBreakout:
		signalDescription = "Bullish Breakout OI↑ | Price↑"
		headerTemplate = "green"
	case models.BearishMomentum:
		signalDescription = "Bearish Momentum OI↑ | Price↓"
		headerTemplate = "red"
	default:
		signalDescription = "Unknown Signal"
		headerTemplate = "grey"
	}

	tradeAction := "Long"
	if signal.SignalType == models.BearishMomentum {
		tradeAction = "Short"
	}

	header := CardHeader{
		Title: CardText{
			Tag:     "plain_text",
			Content: fmt.Sprintf("%s %s Trading Signal", signal.Symbol, signal.SignalType.Emoji()),
		},
		Template: headerTemplate,
	}

	card := LarkCardMessage{
		MsgType: "interactive",
		Card: LarkCard{
			Config: CardConfig{
				WideScreenMode: true,
			},
			Header:   header,
			Elements: []interface{}{
				DivElement{
					Tag: "div",
					Fields: []Field{
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**Time**: %s", signal.Timestamp.Format("01-02 15:04:05"))}},
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**Count**: t%d", signal.AlertsIn24h)}},
					},
				},
				HrElement{Tag: "hr"},
				DivElement{
					Tag:  "div",
					Text: &CardText{Tag: "lark_md", Content: fmt.Sprintf("**%s**", signalDescription)},
				},
				DivElement{
					Tag: "div",
					Fields: []Field{
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**%s**: `%.4f`", tradeAction, signal.CurrentPrice)}},
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**StopLoss**: `%.4f`", signal.StopLoss)}},
					},
				},
				DivElement{
					Tag: "div",
					Fields: []Field{
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**Quantity**: `%.2f`", signal.Quantity)}},
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**ATR**: `%.4f`", signal.ATR)}},
					},
				},
				HrElement{Tag: "hr"},
				DivElement{
					Tag: "div",
					Fields: []Field{
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("📌 **OI Change**: %.2f%%", signal.OIChange)}},
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("📈 **Price Change**: %.2f%%", signal.PriceChange)}},
					},
				},
				DivElement{
					Tag: "div",
					Fields: []Field{
						{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("📊 **ADX(14)**: `%.2f`", signal.ADX)}},
					},
				},
			},
		},
	}
	return card
}

// sendMessage 发送消息到飞书
func (b *Bot) sendMessage(jsonData []byte) error {
	resp, err := b.client.Post(b.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书API错误 [%d]: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if code, ok := result["code"].(float64); ok && code != 0 {
			return fmt.Errorf("飞书API返回错误码: %.0f, 消息: %v", code, result["msg"])
		}
	}

	log.Printf("消息卡片已发送到飞书")
	return nil
}


// ProcessSignals 处理信号通道
func (b *Bot) ProcessSignals(signalCh <-chan models.Signal) {
	for signal := range signalCh {
		if err := b.SendSignal(signal); err != nil {
			log.Printf("发送飞书消息失败: %v", err)
		}
		// 避免触发速率限制
		time.Sleep(1 * time.Second)
	}
}
