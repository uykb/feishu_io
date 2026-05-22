package models

// DataSource 数据源类型
type DataSource string

const (
	SourceBinance     DataSource = "binance"
	SourceHyperliquid DataSource = "hyperliquid"
)

func (d DataSource) String() string {
	switch d {
	case SourceBinance:
		return "Binance"
	case SourceHyperliquid:
		return "Hyperliquid"
	default:
		return "Unknown"
	}
}

func (d DataSource) DisplaySymbol(symbol string) string {
	switch d {
	case SourceHyperliquid:
		return symbol + "USDT"
	default:
		return symbol
	}
}

func (d DataSource) NormalizeSymbol(symbol string) string {
	switch d {
	case SourceHyperliquid:
		if len(symbol) > 4 && len(symbol) > 4 {
			suffix := symbol[len(symbol)-4:]
			if suffix == "USDT" {
				return symbol[:len(symbol)-4]
			}
		}
		return symbol
	default:
		return symbol
	}
}
