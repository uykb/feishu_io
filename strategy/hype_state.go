package strategy

import (
	"binance-monitor/models"
	"sync"
	"time"
)

// HypePhase HYPE状态阶段
type HypePhase int

const (
	PhaseNormal HypePhase = iota
	PhaseDowntrend
	PhasePotentialBottom
	PhaseBottomConfirmed
	PhaseRallying
)

func (p HypePhase) String() string {
	switch p {
	case PhaseNormal:
		return "正常"
	case PhaseDowntrend:
		return "下跌中"
	case PhasePotentialBottom:
		return "底部吸筹"
	case PhaseBottomConfirmed:
		return "底部确认"
	case PhaseRallying:
		return "拉升中"
	default:
		return "未知"
	}
}

// OISnapshot OI快照
type OISnapshot struct {
	OI        float64
	Timestamp time.Time
}

// HypeState HYPE状态追踪
type HypeState struct {
	mu sync.RWMutex

	Phase          HypePhase
	LowestPrice    float64
	LowestPriceTime time.Time
	LowestOI       float64
	LowestOITime   time.Time

	OIHistory []OISnapshot
	FRHistory []float64

	LastSignal     models.HypeSignalType
	LastSignalTime time.Time

	// 用于追踪下跌趋势
	ConsecutiveLowerLows int
	PreviousLow          float64

	// 用于追踪拉升
	RallyStartPrice float64
	RallyStartOI    float64
}

// NewHypeState 创建HYPE状态
func NewHypeState() *HypeState {
	return &HypeState{
		OIHistory: make([]OISnapshot, 0, 360),
		FRHistory: make([]float64, 0, 120),
	}
}

// UpdateOI 更新OI快照
func (hs *HypeState) UpdateOI(oi float64, ts time.Time) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.OIHistory = append(hs.OIHistory, OISnapshot{OI: oi, Timestamp: ts})

	// 保留最近60分钟的数据（假设每10秒一个点，最多360个）
	if len(hs.OIHistory) > 360 {
		hs.OIHistory = hs.OIHistory[1:]
	}
}

// UpdateFundingRate 更新资金费率
func (hs *HypeState) UpdateFundingRate(rate float64) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.FRHistory = append(hs.FRHistory, rate)

	// 保留最近60分钟的数据（假设每30秒一个点，最多120个）
	if len(hs.FRHistory) > 120 {
		hs.FRHistory = hs.FRHistory[1:]
	}
}

// UpdatePrice 更新价格并追踪最低点
func (hs *HypeState) UpdatePrice(price float64, ts time.Time) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.LowestPrice == 0 || price < hs.LowestPrice {
		hs.LowestPrice = price
		hs.LowestPriceTime = ts
	}
}

// GetRecentOIChange 获取最近N分钟的OI变化率
func (hs *HypeState) GetRecentOIChange(minutes int) (float64, bool) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.OIHistory) < 2 {
		return 0, false
	}

	now := hs.OIHistory[len(hs.OIHistory)-1].Timestamp
	cutoff := now.Add(-time.Duration(minutes) * time.Minute)

	// 找到时间窗口内的第一个和最后一个OI
	var first, last OISnapshot
	found := false
	for i := len(hs.OIHistory) - 1; i >= 0; i-- {
		if !found && hs.OIHistory[i].Timestamp.Before(cutoff) {
			first = hs.OIHistory[i]
			found = true
		}
		if hs.OIHistory[i].Timestamp.After(cutoff) {
			last = hs.OIHistory[i]
		}
	}

	if !found || first.OI == 0 {
		// 如果找不到足够的数据，用全部历史
		first = hs.OIHistory[0]
		last = hs.OIHistory[len(hs.OIHistory)-1]
		if first.OI == 0 {
			return 0, false
		}
	}

	change := (last.OI - first.OI) / first.OI * 100
	return change, true
}

// GetOIHistory 获取最近N个OI快照
func (hs *HypeState) GetOIHistory(count int) []OISnapshot {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.OIHistory) <= count {
		return hs.OIHistory
	}
	return hs.OIHistory[len(hs.OIHistory)-count:]
}

// GetCurrentOI 获取当前OI
func (hs *HypeState) GetCurrentOI() float64 {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.OIHistory) == 0 {
		return 0
	}
	return hs.OIHistory[len(hs.OIHistory)-1].OI
}

// GetRecentFRChange 获取最近N分钟资金费率变化
func (hs *HypeState) GetRecentFRChange(minutes int) (float64, float64, bool) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.FRHistory) < 2 {
		return 0, 0, false
	}

	cutoffIdx := len(hs.FRHistory) - 1
	for i := len(hs.FRHistory) - 1; i >= 0; i-- {
		cutoffIdx = i
		if i < len(hs.FRHistory)-minutes*2 {
			break
		}
	}
	if cutoffIdx < 0 {
		cutoffIdx = 0
	}

	oldRate := hs.FRHistory[cutoffIdx]
	newRate := hs.FRHistory[len(hs.FRHistory)-1]

	return newRate, oldRate, true
}

// GetCurrentFR 获取当前资金费率
func (hs *HypeState) GetCurrentFR() float64 {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.FRHistory) == 0 {
		return 0
	}
	return hs.FRHistory[len(hs.FRHistory)-1]
}

// CanSendSignal 检查是否可以发送信号（冷却机制）
func (hs *HypeState) CanSendSignal(signalType models.HypeSignalType, cooldown time.Duration) bool {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if hs.LastSignal == signalType {
		return time.Since(hs.LastSignalTime) > cooldown
	}
	return true
}

// RecordSignal 记录已发送的信号
func (hs *HypeState) RecordSignal(signalType models.HypeSignalType) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.LastSignal = signalType
	hs.LastSignalTime = time.Now()
}

// GetPhase 获取当前阶段
func (hs *HypeState) GetPhase() HypePhase {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.Phase
}

// SetPhase 设置阶段
func (hs *HypeState) SetPhase(phase HypePhase) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.Phase = phase
}

// GetLowestPrice 获取最低价格
func (hs *HypeState) GetLowestPrice() float64 {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return hs.LowestPrice
}

// ResetLowestPrice 重置最低价格（用于新阶段）
func (hs *HypeState) ResetLowestPrice(price float64, ts time.Time) {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.LowestPrice = price
	hs.LowestPriceTime = ts
}

// GetFRHistory 获取最近N个资金费率
func (hs *HypeState) GetFRHistory(count int) []float64 {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	if len(hs.FRHistory) <= count {
		return hs.FRHistory
	}
	return hs.FRHistory[len(hs.FRHistory)-count:]
}
