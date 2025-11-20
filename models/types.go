package models

import "time"

// KlineData 15分钟K线数据
type KlineData struct {
	Symbol    string
	Timestamp time.Time
	Open      float64
	Close     float64
	High      float64
	Low       float64
}

// OIData 持仓量数据
type OIData struct {
	Symbol         string
	OpenInterest   float64
	Timestamp      time.Time
}

// MarketData 市场数据汇总
type MarketData struct {
	Symbol           string
	PriceChange      float64
	OIChange         float64
	CurrentPrice     float64
	CurrentOI        float64
	PreviousPrice    float64
	PreviousOI       float64
	Timestamp        time.Time
}

// Signal 交易信号
type Signal struct {
	Symbol       string
	SignalType   SignalType
	PriceChange  float64
	OIChange     float64
	CurrentPrice float64
	CurrentOI    float64
	Timestamp    time.Time
	ATR          float64 // 平均真实波幅
	StopLoss     float64 // 建议止损价
	Quantity     float64 // 建议仓位大小
}

// SignalType 信号类型
type SignalType int

const (
	BullishBreakout    SignalType = iota // OI↑ + Price↑ = 看涨突破
	BearishMomentum                      // OI↑ + Price↓ = 看跌动量
	PossibleFakeout                      // OI↓ + Price↑ = 可能假突破
	MarketContraction                    // OI↓ + Price↓ = 市场收缩
)

func (st SignalType) String() string {
	switch st {
	case BullishBreakout:
		return "看涨突破 (Bullish Breakout)"
	case BearishMomentum:
		return "看跌动量 (Bearish Momentum)"
	case PossibleFakeout:
		return "可能假突破 (Possible Fakeout)"
	case MarketContraction:
		return "市场收缩 (Market Contraction)"
	default:
		return "未知信号"
	}
}

func (st SignalType) Emoji() string {
	switch st {
	case BullishBreakout:
		return "🔴"
	case BearishMomentum:
		return "🟢"
	case PossibleFakeout:
		return "🟡"
	case MarketContraction:
		return "🔵"
	default:
		return "⚪"
	}
}
