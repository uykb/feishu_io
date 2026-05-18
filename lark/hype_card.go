package lark

import (
	"fmt"

	"binance-monitor/models"
)

// HypeCardElement 飞书卡片元素类型别名
type HypeCardElement = interface{}

// FormatHypeCard 格式化HYPE信号为飞书卡片
func FormatHypeCard(signal models.HypeSignal) LarkCardMessage {
	header := CardHeader{
		Title: CardText{
			Tag:     "plain_text",
			Content: fmt.Sprintf("%s %s %s", signal.Symbol, signal.SignalType.Emoji(), signal.SignalType.String()),
		},
		Template: signal.SignalType.HeaderTemplate(),
	}

	elements := buildHypeElements(signal)

	return LarkCardMessage{
		MsgType: "interactive",
		Card: LarkCard{
			Config: CardConfig{
				WideScreenMode: true,
			},
			Header:   header,
			Elements: elements,
		},
	}
}

// buildHypeElements 构建HYPE卡片元素
func buildHypeElements(signal models.HypeSignal) []HypeCardElement {
	elements := []HypeCardElement{}

	elements = append(elements, DivElement{
		Tag: "div",
		Fields: []Field{
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**时间**: %s", signal.Timestamp.Format("01-02 15:04:05"))}},
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**阶段**: %s", signal.SignalType.String())}},
		},
	})

	elements = append(elements, HrElement{Tag: "hr"})

	elements = append(elements, DivElement{
		Tag:  "div",
		Text: &CardText{Tag: "lark_md", Content: fmt.Sprintf("**%s**", signal.Description)},
	})

	elements = append(elements, DivElement{
		Tag: "div",
		Fields: []Field{
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**价格**: `$%.2f`", signal.Price)}},
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**最低价**: `$%.2f`", signal.LowestPrice)}},
		},
	})

	if signal.PriceChange != 0 {
		changeIcon := "📈"
		if signal.PriceChange < 0 {
			changeIcon = "📉"
		}
		elements = append(elements, DivElement{
			Tag: "div",
			Fields: []Field{
				{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("%s **价格变化**: `%.2f%%`", changeIcon, signal.PriceChange)}},
			},
		})
	}

	elements = append(elements, HrElement{Tag: "hr"})

	oiIcon := "📊"
	if signal.OIChange > 0 {
		oiIcon = "📈"
	} else if signal.OIChange < 0 {
		oiIcon = "📉"
	}

	elements = append(elements, DivElement{
		Tag: "div",
		Fields: []Field{
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("%s **OI**: `%.0f`", oiIcon, signal.CurrentOI)}},
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**OI趋势**: %s (`%.2f%%`)", signal.OITrend, signal.OIChange)}},
		},
	})

	frStatus := formatFRStatus(signal.FundingRate, signal.FRStatus)
	elements = append(elements, DivElement{
		Tag: "div",
		Fields: []Field{
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("💰 **资金费率**: `%.5f%%`", signal.FundingRate*100)}},
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("**状态**: %s", frStatus)}},
		},
	})

	if signal.FRChange != 0 {
		elements = append(elements, DivElement{
			Tag: "div",
			Fields: []Field{
				{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("🔄 **费率变化**: `%.1f%%`", signal.FRChange)}},
			},
		})
	}

	if signal.ADX > 0 {
		elements = append(elements, DivElement{
			Tag: "div",
			Fields: []Field{
				{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("📊 **ADX(14)**: `%.2f`", signal.ADX)}},
			},
		})
	}

	elements = append(elements, HrElement{Tag: "hr"})

	elements = append(elements, DivElement{
		Tag: "div",
		Fields: []Field{
			{IsShort: true, Text: CardText{Tag: "lark_md", Content: fmt.Sprintf("💡 **建议**: %s", signal.Action)}},
		},
	})

	return elements
}

// formatFRStatus 格式化资金费率状态
func formatFRStatus(rate float64, status string) string {
	if status != "" {
		return status
	}

	ratePct := rate * 100
	if ratePct > 0.001 {
		return "🟢 多头拥挤"
	} else if ratePct > 0.0005 {
		return "🟡 多头偏多"
	} else if ratePct > 0 {
		return "⚪ 轻微偏多"
	} else if ratePct == 0 {
		return "⚪ 中性"
	} else if ratePct > -0.0002 {
		return "⚪ 轻微偏空"
	} else if ratePct > -0.0005 {
		return "🟡 空头偏空"
	} else if ratePct > -0.001 {
		return "🔴 空头拥挤"
	} else {
		return "🔴 极度拥挤"
	}
}
