package models

import "time"

// FundingRateData 资金费率数据
type FundingRateData struct {
	Symbol      string
	FundingRate float64
	FundingTime time.Time
	Timestamp   time.Time
}

// HypeSignalType HYPE信号类型
type HypeSignalType int

const (
	DowntrendAccelerating HypeSignalType = iota // 加速下跌预警
	PotentialBottom                              // 底部吸筹信号
	BottomConfirmed                              // Higher Low 底部确认
	ShortSqueezeRally                            // 轧空拉升
	TrendReversal                                // 趋势反转确认
)

func (st HypeSignalType) String() string {
	switch st {
	case DowntrendAccelerating:
		return "加速下跌"
	case PotentialBottom:
		return "底部吸筹"
	case BottomConfirmed:
		return "底部确认"
	case ShortSqueezeRally:
		return "轧空拉升"
	case TrendReversal:
		return "趋势反转"
	default:
		return "未知信号"
	}
}

func (st HypeSignalType) Emoji() string {
	switch st {
	case DowntrendAccelerating:
		return "📉"
	case PotentialBottom:
		return "🔍"
	case BottomConfirmed:
		return "✅"
	case ShortSqueezeRally:
		return "🚀"
	case TrendReversal:
		return "📈"
	default:
		return "⚪"
	}
}

func (st HypeSignalType) HeaderTemplate() string {
	switch st {
	case DowntrendAccelerating:
		return "red"
	case PotentialBottom:
		return "orange"
	case BottomConfirmed:
		return "blue"
	case ShortSqueezeRally:
		return "turquoise"
	case TrendReversal:
		return "green"
	default:
		return "grey"
	}
}

// HypeSignal HYPE专属交易信号
type HypeSignal struct {
	Symbol      string
	SignalType  HypeSignalType
	Price       float64
	PriceChange float64
	LowestPrice float64
	CurrentOI   float64
	OIChange    float64
	OITrend     string
	FundingRate float64
	FRChange    float64
	FRStatus    string
	ADX         float64
	Description string
	Action      string
	Timestamp   time.Time
}
