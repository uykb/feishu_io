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
func NewBot(webhookURL string) *Bot {
	return &Bot{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// LarkMessage 飞书消息结构
type LarkMessage struct {
	MsgType string      `json:"msg_type"`
	Content LarkContent `json:"content"`
}

type LarkContent struct {
	Text string `json:"text"`
}

// SendSignal 发送交易信号
func (b *Bot) SendSignal(signal models.Signal) error {
	message := b.formatMessage(signal)
	return b.sendMessage(message)
}

// formatMessage 格式化消息
func (b *Bot) formatMessage(signal models.Signal) string {
	var signalDescription string
	switch signal.SignalType {
	case models.BullishBreakout:
		signalDescription = "看涨突破 OI↑|Price↑"
	case models.BearishMomentum:
		signalDescription = "看跌动量 OI↑|Price↓"
	case models.PossibleFakeout:
		signalDescription = "可能假突破 OI↓|Price↑"
	case models.MarketContraction:
		signalDescription = "市场收缩 OI↓|Price↓"
	default:
		signalDescription = "未知信号"
	}

	// 根据信号类型决定是买入还是卖出
	tradeAction := "Buy"
	if signal.SignalType == models.BearishMomentum || signal.SignalType == models.MarketContraction {
		tradeAction = "Sell"
	}

	message := fmt.Sprintf(`%s%s %s\n⏰ %s (周期: 15)\n⚠️ 交易建议(ATR:%.4f)\n%s:%.4f  SL:%.4f Qty:%.2f\n📌%.2f%% OI + %.2f%% Price`,
		signal.Symbol,
		signal.SignalType.Emoji(),
		signalDescription,
		signal.Timestamp.Format("01-02 15:04:05"),
		signal.ATR,
		tradeAction,
		signal.CurrentPrice,
		signal.StopLoss,
		signal.Quantity,
		signal.OIChange,
		signal.PriceChange,
	)

	return message
}

// sendMessage 发送消息到飞书
func (b *Bot) sendMessage(text string) error {
	msg := LarkMessage{
		MsgType: "text",
		Content: LarkContent{
			Text: text,
		},
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	resp, err := b.client.Post(b.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("发送消息失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("飞书API错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 检查响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if code, ok := result["code"].(float64); ok && code != 0 {
			return fmt.Errorf("飞书API返回错误码: %.0f, 消息: %v", code, result["msg"])
		}
	}

	log.Printf("消息已发送到飞书: %s", text[:min(50, len(text))])
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
