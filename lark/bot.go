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
	// 获取信号类型说明
	var interpretation, advice string

	switch signal.SignalType {
	case models.BullishBreakout:
		interpretation = "OI和价格同时大幅上升，表明强劲的看涨突破信号。"
		advice = `• 结合支撑位确认入场点
• 止损设在关键支撑下方
• 目标看向近期阻力位`
	case models.BearishMomentum:
		interpretation = "OI上升但价格下跌，表明看跌动量增强。"
		advice = `• 结合阻力位确认入场点
• 止损设在关键阻力上方
• 目标看向近期支撑位`
	case models.PossibleFakeout:
		interpretation = "价格上升但OI下降，可能是假突破信号，需谨慎。"
		advice = `• 观察价格是否能站稳
• 等待OI确认再入场
• 设置较紧止损`
	case models.MarketContraction:
		interpretation = "OI和价格同时下降，市场进入收缩阶段。"
		advice = `• 观察市场方向选择
• 等待明确突破信号
• 避免盲目入场`
	}

	// OI变化方向
	oiDirection := "增加"
	if signal.OIChange < 0 {
		oiDirection = "减少"
	}

	// 价格变化方向
	priceDirection := "强劲买压，潜在上涨趋势"
	if signal.PriceChange < 0 {
		priceDirection = "卖压，可能趋势反转"
	}

	message := fmt.Sprintf(`%s 合约OI入场信号触发 - %s %s

⏰ 时间: %s (周期: 15)

📊 当前数据
🔴 OI变化: %.16f%% (阈值: >5%%)
🟢 价格变化: %.16f%% (阈值: >2%%)

💡 市场解读
当前组合: %.2f%% OI + %.2f%% Price

• OI↑ + Price↑ = 看涨突破 (Bullish Breakout)
• OI↑ + Price↓ = 看跌动量 (Bearish Momentum)
• OI↓ + Price↑ = 可能假突破 (Possible Fakeout)
• OI↓ + Price↓ = 市场收缩 (Market Contraction)

📈 信号意义总结
警报自动触发，无需手动设置！
✅ 当OI变化(15m)超过阈值时通知
✅ 当价格变化(15m)超过阈值的通知
✅ 显示OI和价格的确切百分比变化

- OI变化阈值过去15分钟交易量变化:
  • %s → 更多交易者建仓 (市场活动增加)
  • 减少 → 交易者平仓 (潜在反转或盘整)

- 价格变化显示过去15分钟价格波动:
  • 增加 → %s
  • 减少 → 卖压，可能趋势反转

- 入场信号同时满足OI>5%%和价格>2%%时触发:

  📊 信号解读
%s

⚠️ 交易建议
%s

📌 信号强度: %.2f%% OI + %.2f%% Price`,
		signal.Symbol,
		signal.SignalType.Emoji(),
		signal.SignalType.String(),
		signal.Timestamp.Format("2006-01-02T15:04:05Z"),
		signal.OIChange,
		signal.PriceChange,
		signal.OIChange,
		signal.PriceChange,
		oiDirection,
		priceDirection,
		interpretation,
		advice,
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
