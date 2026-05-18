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
}

// HypeDetector HYPE专属信号检测器
type HypeDetector struct {
	config HypeDetectorConfig
	state  *HypeState
	signalCh chan models.HypeSignal
	symbol   string
}

// NewHypeDetector 创建HYPE检测器
func NewHypeDetector(symbol string, config HypeDetectorConfig, signalCh chan models.HypeSignal) *HypeDetector {
	return &HypeDetector{
		config:   config,
		state:    NewHypeState(),
		signalCh: signalCh,
		symbol:   symbol,
	}
}

// ProcessKlineData 处理K线数据
func (hd *HypeDetector) ProcessKlineData(klineDataCh <-chan models.KlineData) {
	for kline := range klineDataCh {
		if kline.Symbol != hd.symbol {
			continue
		}

		hd.state.UpdatePrice(kline.Low, kline.Timestamp)
		hd.checkSignals(kline)
	}
}

// ProcessOIData 处理OI数据
func (hd *HypeDetector) ProcessOIData(oiDataCh <-chan models.OIData) {
	for oiData := range oiDataCh {
		if oiData.Symbol != hd.symbol {
			continue
		}

		hd.state.UpdateOI(oiData.OpenInterest, oiData.Timestamp)
	}
}

// ProcessFundingRate 处理资金费率数据
func (hd *HypeDetector) ProcessFundingRate(fundingCh <-chan models.FundingRateData) {
	for frData := range fundingCh {
		if frData.Symbol != hd.symbol {
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

	// 检查是否连续创新低
	lowestPrice := hd.state.GetLowestPrice()
	isNewLow := price <= lowestPrice

	if isNewLow {
		hd.state.SetPhase(PhaseDowntrend)
	}

	if !isNewLow && hd.state.GetPhase() == PhaseDowntrend {
		hd.state.ConsecutiveLowerLows = 0
	}

	// 信号1: DowntrendAccelerating - 加速下跌预警
	if hd.checkDowntrendAccelerating(price, oiChange30m, fr, adx) {
		return
	}

	// 信号2: PotentialBottom - 底部吸筹
	if hd.checkPotentialBottom(price, oi, oiChange30m, fr, adx, kline.Timestamp) {
		return
	}
}

// checkDowntrendAccelerating 检查加速下跌
func (hd *HypeDetector) checkDowntrendAccelerating(price, oiChange30m, fr, adx float64) bool {
	if oiChange30m >= -0.1 {
		return false
	}

	if fr <= 0.0005 {
		return false
	}

	if adx < 15 {
		return false
	}

	cooldown := time.Duration(hd.config.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.DowntrendAccelerating, cooldown) {
		return false
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
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
	lowestPrice := hd.state.GetLowestPrice()
	isAtLow := price <= lowestPrice*(1+hd.config.HigherLowPct/100)

	if !isAtLow {
		return false
	}

	if oiChange30m > hd.config.OIStopThreshold {
		return false
	}

	if fr > hd.config.FRExtremeThreshold {
		return false
	}

	if adx < 15 {
		return false
	}

	cooldown := time.Duration(hd.config.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.PotentialBottom, cooldown) {
		return false
	}

	frStatus := "正常"
	if fr < -0.001 {
		frStatus = "极度拥挤"
	} else if fr < -0.0005 {
		frStatus = "拥挤"
	} else if fr < -0.0002 {
		frStatus = "偏空"
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
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
	lowestPrice := hd.state.GetLowestPrice()

	if lowestPrice == 0 {
		return
	}

	higherLowThreshold := lowestPrice * (1 + hd.config.HigherLowPct/100)

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

	if frRecovery < hd.config.FRRecoveryThreshold {
		return
	}

	cooldown := time.Duration(hd.config.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.BottomConfirmed, cooldown) {
		return
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
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

	if priceChange < hd.config.SqueezePricePct {
		return
	}

	if oiDecline > -hd.config.SqueezeOIDeclinePct {
		return
	}

	currentFR, oldFR, hasFR := hd.state.GetRecentFRChange(30)
	frRecovery := 0.0
	if hasFR && oldFR < 0 {
		frRecovery = (currentFR - oldFR) / (-oldFR)
	}

	if frRecovery < 0.3 && fr > 0 {
	}

	cooldown := time.Duration(hd.config.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.ShortSqueezeRally, cooldown) {
		return
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
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

	cooldown := time.Duration(hd.config.CooldownMinutes) * time.Minute
	if !hd.state.CanSendSignal(models.TrendReversal, cooldown) {
		return
	}

	signal := models.HypeSignal{
		Symbol:      hd.symbol,
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

// emitSignal 发送信号
func (hd *HypeDetector) emitSignal(signal models.HypeSignal) {
	select {
	case hd.signalCh <- signal:
		log.Printf("[HYPE] 触发信号: %s %s (价格: $%.2f, OI: %.0f, 费率: %.5f%%)",
			hd.symbol, signal.SignalType.String(), signal.Price, signal.CurrentOI, signal.FundingRate*100)
	default:
		log.Printf("[HYPE] 信号通道已满，丢弃 %s 信号", hd.symbol)
	}
}
