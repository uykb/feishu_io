package strategy

import (
	"binance-monitor/models"
	"log"
	"time"
)

// HypeDetectorConfig HYPE检测器配置
type HypeDetectorConfig struct {
	OIStopThreshold      float64
	FRExtremeThreshold   float64
	FRRecoveryThreshold  float64
	HigherLowPct         float64
	SqueezePricePct      float64
	SqueezeOIDeclinePct  float64
	CooldownMinutes      int
	LookbackKlines       int
	ADXThreshold         float64
	ADXPeriod            int
	ATRPeriod            int
	// Hyperliquid-specific thresholds
	HLOIStopThreshold    float64
	HLFRExtremeThreshold float64
	HLFRRecoveryThreshold float64
	HLSqueezePricePct    float64
	HLSqueezeOIDeclinePct float64
}

// HypeDetector HYPE专属信号检测器
type HypeDetector struct {
	config   HypeDetectorConfig
	state    *HypeState
	signalCh chan models.HypeSignal
	symbol   string
	source   models.DataSource
}

// NewHypeDetector 创建HYPE检测器
func NewHypeDetector(symbol string, source models.DataSource, config HypeDetectorConfig, signalCh chan models.HypeSignal) *HypeDetector {
	return &HypeDetector{
		config:   config,
		state:    NewHypeState(),
		signalCh: signalCh,
		symbol:   symbol,
		source:   source,
	}
}

// thresholds 根据数据源返回对应阈值
func (hd *HypeDetector) thresholds() HypeDetectorConfig {
	if hd.source == models.SourceHyperliquid {
		return HypeDetectorConfig{
			OIStopThreshold:      hd.config.HLOIStopThreshold,
			FRExtremeThreshold:   hd.config.HLFRExtremeThreshold,
			FRRecoveryThreshold:  hd.config.HLFRRecoveryThreshold,
			SqueezePricePct:      hd.config.HLSqueezePricePct,
			SqueezeOIDeclinePct:  hd.config.HLSqueezeOIDeclinePct,
			HigherLowPct:         hd.config.HigherLowPct,
			CooldownMinutes:      hd.config.CooldownMinutes,
			LookbackKlines:       hd.config.LookbackKlines,
			ADXThreshold:         hd.config.ADXThreshold,
			ADXPeriod:            hd.config.ADXPeriod,
			ATRPeriod:            hd.config.ATRPeriod,
		}
	}
	return hd.config
}

// ProcessKlineData 处理K线数据
func (hd *HypeDetector) ProcessKlineData(klineDataCh <-chan models.KlineData) {
	for kline := range klineDataCh {
		if kline.Symbol != hd.symbol || kline.Source != hd.source {
			continue
		}

		hd.state.UpdatePrice(kline.Low, kline.Timestamp)
		hd.checkSignals(kline)
	}
}

// ProcessOIData 处理OI数据
func (hd *HypeDetector) ProcessOIData(oiDataCh <-chan models.OIData) {
	for oiData := range oiDataCh {
		if oiData.Symbol != hd.symbol || oiData.Source != hd.source {
			continue
		}

		hd.state.UpdateOI(oiData.OpenInterest, oiData.Timestamp)
	}
}

// ProcessFundingRate 处理资金费率数据
func (hd *HypeDetector) ProcessFundingRate(fundingCh <-chan models.FundingRateData) {
	for frData := range fundingCh {
		if frData.Symbol != hd.symbol || frData.Source != hd.source {
			continue
		}

		hd.state.UpdateFundingRate(frData.FundingRate)
	}
}

// checkSignals 检查所有信号
func (hd *HypeDetector) checkSignals(kline models.KlineData) {
	currentPrice := kline.Close
	currentOI := hd.state.GetCurrentOI()
	currentFR := hd.state.GetCurrentFR()
	adx := hd.calculateADX(kline)

	phase := hd.state.GetPhase()

	switch phase {
	case PhaseNormal, PhaseDowntrend:
		hd.checkDowntrendAndBottom(kline, currentPrice, currentOI, currentFR, adx)
	case PhasePotentialBottom:
		hd.checkBottomConfirmed(kline, currentPrice, currentOI, currentFR, adx)
	case PhaseBottomConfirmed, PhaseRallying:
		hd.checkRallyAndReversal(kline, currentPrice, currentOI, currentFR, adx)
	}
}

// checkDowntrendAndBottom 检查下跌趋势和底部信号
func (hd *HypeDetector) checkDowntrendAndBottom(kline models.KlineData, price, oi, fr, adx float64) {
	oiChange30m, hasOI := hd.state.GetRecentOIChange(30)
	if !hasOI || oi == 0 {
		return
	}

	lowestPrice := hd.state.GetLowestPrice()
	isNewLow := price <= lowestPrice

	if isNewLow {
		hd.state.SetPhase(PhaseDowntrend)
	}

	if !isNewLow && hd.state.GetPhase() == PhaseDowntrend {
		hd.state.ConsecutiveLowerLows = 0
	}

	if hd.checkDowntrendAccelerating(price, oiChange30m, fr, adx) {
		return
	}

	if hd.checkPotentialBottom(price, oi, oiChange30m, fr, adx, kline.Timestamp) {
		return
	}
}

// checkDowntrendAccelerating 检查加速下跌
func (hd *HypeDetector) checkDowntrendAccelerating(price, oiChange30m, fr, adx float64) bool {
	t := hd.thresholds()

	frThreshold := 0.0005
	if hd.source == models.SourceHyperliquid {
		frThreshold = 0.00005
	}

	if oiChange30m >= -0.1 {
		return false
	}

	if fr <= frThreshold {
		return false
	}

	if adx < 15 {
		return false
	}

	cooldown := time.Duration(t.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.DowntrendAccelerating, cooldown) {
		return false
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
		Source:      hd.source,
		SignalType:  models.DowntrendAccelerating,
		Price:       price,
		CurrentOI:   hd.state.GetCurrentOI(),
		OIChange:    oiChange30m,
		OITrend:     "持续下降",
		FundingRate: fr,
		FRStatus:    "多头拥挤",
		ADX:         adx,
		Description: "价格阶梯下行，OI持续下降多头止损离场",
		Action:      "避免抄底，等待底部信号",
		Timestamp:   time.Now(),
	}

	hd.emitSignal(signal)
	hd.state.RecordSignal(models.DowntrendAccelerating)
	return true
}

// checkPotentialBottom 检查底部吸筹信号
func (hd *HypeDetector) checkPotentialBottom(price, oi, oiChange30m, fr, adx float64, ts time.Time) bool {
	t := hd.thresholds()

	lowestPrice := hd.state.GetLowestPrice()
	isAtLow := price <= lowestPrice*(1+t.HigherLowPct/100)

	if !isAtLow {
		return false
	}

	if oiChange30m > t.OIStopThreshold {
		return false
	}

	if fr > t.FRExtremeThreshold {
		return false
	}

	if adx < 15 {
		return false
	}

	cooldown := time.Duration(t.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.PotentialBottom, cooldown) {
		return false
	}

	frStatus := hd.formatFRStatus(fr)

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
		Source:      hd.source,
		SignalType:  models.PotentialBottom,
		Price:       price,
		LowestPrice: lowestPrice,
		CurrentOI:   oi,
		OIChange:    oiChange30m,
		OITrend:     "止跌/微增",
		FundingRate: fr,
		FRChange:    0,
		FRStatus:    frStatus,
		ADX:         adx,
		Description: "价格创新低但OI止跌，资金费率极负，空头拥挤",
		Action:      "观望，等待Higher Low确认",
		Timestamp:   time.Now(),
	}

	hd.emitSignal(signal)
	hd.state.SetPhase(PhasePotentialBottom)
	hd.state.RecordSignal(models.PotentialBottom)
	return true
}

// checkBottomConfirmed 检查Higher Low确认
func (hd *HypeDetector) checkBottomConfirmed(kline models.KlineData, price, oi, fr, adx float64) {
	t := hd.thresholds()
	lowestPrice := hd.state.GetLowestPrice()

	if lowestPrice == 0 {
		return
	}

	higherLowThreshold := lowestPrice * (1 + t.HigherLowPct/100)

	if price < higherLowThreshold {
		return
	}

	if price > lowestPrice*1.02 {
		return
	}

	oiChange30m, hasOI := hd.state.GetRecentOIChange(30)
	if !hasOI {
		return
	}

	currentFR, oldFR, hasFR := hd.state.GetRecentFRChange(30)
	if !hasFR {
		return
	}

	frRecovery := 0.0
	if oldFR < 0 {
		frRecovery = (currentFR - oldFR) / (-oldFR)
	}

	if frRecovery < t.FRRecoveryThreshold {
		return
	}

	cooldown := time.Duration(t.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.BottomConfirmed, cooldown) {
		return
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
		Source:      hd.source,
		SignalType:  models.BottomConfirmed,
		Price:       price,
		LowestPrice: lowestPrice,
		CurrentOI:   oi,
		OIChange:    oiChange30m,
		OITrend:     "稳定",
		FundingRate: currentFR,
		FRChange:    frRecovery * 100,
		FRStatus:    "费率收窄",
		ADX:         adx,
		Description: "Higher Low确认，资金费率从极负向0靠近",
		Action:      "可轻仓试多，止损前低",
		Timestamp:   time.Now(),
	}

	hd.emitSignal(signal)
	hd.state.SetPhase(PhaseBottomConfirmed)
	hd.state.RecordSignal(models.BottomConfirmed)
}

// checkRallyAndReversal 检查轧空拉升和趋势反转
func (hd *HypeDetector) checkRallyAndReversal(kline models.KlineData, price, oi, fr, adx float64) {
	phase := hd.state.GetPhase()

	if phase == PhaseBottomConfirmed {
		hd.checkShortSqueeze(kline, price, oi, fr, adx)
	}

	if phase == PhaseRallying {
		hd.checkTrendReversal(kline, price, oi, fr, adx)
	}
}

// checkShortSqueeze 检查轧空拉升
func (hd *HypeDetector) checkShortSqueeze(kline models.KlineData, price, oi, fr, adx float64) {
	t := hd.thresholds()

	oiHistory := hd.state.GetOIHistory(6)
	if len(oiHistory) < 2 {
		return
	}

	startOI := oiHistory[0].OI
	if startOI == 0 {
		return
	}

	oiDecline := (oi - startOI) / startOI * 100

	priceChange := (price - kline.Open) / kline.Open * 100

	if priceChange < t.SqueezePricePct {
		return
	}

	if oiDecline > -t.SqueezeOIDeclinePct {
		return
	}

	currentFR, oldFR, hasFR := hd.state.GetRecentFRChange(30)
	frRecovery := 0.0
	if hasFR && oldFR < 0 {
		frRecovery = (currentFR - oldFR) / (-oldFR)
	}

	cooldown := time.Duration(t.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.ShortSqueezeRally, cooldown) {
		return
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
		Source:      hd.source,
		SignalType:  models.ShortSqueezeRally,
		Price:       price,
		PriceChange: priceChange,
		CurrentOI:   oi,
		OIChange:    oiDecline,
		OITrend:     "下降(空头平仓)",
		FundingRate: currentFR,
		FRChange:    frRecovery * 100,
		FRStatus:    "费率回升",
		ADX:         adx,
		Description: "价格拉升但OI下降，空头平仓驱动的轧空行情",
		Action:      "持有仓位，关注费率转正",
		Timestamp:   time.Now(),
	}

	hd.emitSignal(signal)
	hd.state.SetPhase(PhaseRallying)
	hd.state.RecordSignal(models.ShortSqueezeRally)
}

// checkTrendReversal 检查趋势反转
func (hd *HypeDetector) checkTrendReversal(kline models.KlineData, price, oi, fr, adx float64) {
	t := hd.thresholds()

	oiChange60m, hasOI := hd.state.GetRecentOIChange(60)
	if !hasOI {
		return
	}

	if oiChange60m < 0.05 {
		return
	}

	if fr <= 0 {
		return
	}

	if adx < 20 {
		return
	}

	cooldown := time.Duration(t.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.TrendReversal, cooldown) {
		return
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
		Source:      hd.source,
		SignalType:  models.TrendReversal,
		Price:       price,
		PriceChange: (price - kline.Open) / kline.Open * 100,
		CurrentOI:   oi,
		OIChange:    oiChange60m,
		OITrend:     "回升(多头建仓)",
		FundingRate: fr,
		FRStatus:    "多头回归",
		ADX:         adx,
		Description: "OI回升多头入场，资金费率持续为正，趋势反转确认",
		Action:      "可加仓，目标看前高",
		Timestamp:   time.Now(),
	}

	hd.emitSignal(signal)
	hd.state.RecordSignal(models.TrendReversal)
}

// calculateADX 计算ADX
func (hd *HypeDetector) calculateADX(currentKline models.KlineData) float64 {
	history := make([]models.KlineData, 0, hd.config.LookbackKlines*2+5)

	history = append(history, currentKline)

	if len(history) < hd.config.ADXPeriod*2 {
		return 20
	}

	return CalculateADX(history, hd.config.ADXPeriod)
}

// formatFRStatus 格式化资金费率状态（按数据源）
func (hd *HypeDetector) formatFRStatus(rate float64) string {
	if hd.source == models.SourceHyperliquid {
		ratePct := rate * 100
		if ratePct > 0.0001 {
			return "🟢 多头拥挤"
		} else if ratePct > 0.00005 {
			return "🟡 多头偏多"
		} else if ratePct > 0 {
			return "⚪ 轻微偏多"
		} else if ratePct == 0 {
			return "⚪ 中性"
		} else if ratePct > -0.00002 {
			return "⚪ 轻微偏空"
		} else if ratePct > -0.00005 {
			return "🟡 空头偏空"
		} else if ratePct > -0.0001 {
			return "🔴 空头拥挤"
		} else {
			return "🔴 极度拥挤"
		}
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

// emitSignal 发送信号
func (hd *HypeDetector) emitSignal(signal models.HypeSignal) {
	select {
	case hd.signalCh <- signal:
		log.Printf("[HYPE][%s] 触发信号: %s %s (价格: $%.2f, OI: %.0f, 费率: %.5f%%)",
			hd.source.String(), hd.symbol, signal.SignalType.String(), signal.Price, signal.CurrentOI, signal.FundingRate*100)
	default:
		log.Printf("[HYPE] 信号通道已满，丢弃 %s 信号", hd.symbol)
	}
}
