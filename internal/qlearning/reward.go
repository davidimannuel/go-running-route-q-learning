package qlearning

// Active reference: rute_lari_core.py (3 November 2026). These are research
// preferences, not measured comfort or road-safety standards. Keep direction
// distance Haversine in Go, as explicitly agreed; the reference uses UTM.
const (
	ReturnPhaseRatio         = 0.50
	OutwardPenaltyScale      = 0.60
	ReturnRewardScale        = 0.60
	PermissivePenaltyPer100M = -0.10
	RevisitPenalty           = -10.0
	SuccessBonus             = 100.0
	OverDistancePenalty      = -50.0
	NoActionPenalty          = -50.0
	StepLimitPenalty         = -50.0
)

// RewardComponents records values from the actual transition, not a second
// calculation for reporting. NoAction is separate to avoid double penalizing
// a dead end reached after moving. Closing to start never receives Revisit.
type RewardComponents struct {
	Edge, Access, Direction, Revisit, Terminal, NoAction float64
}

func (r RewardComponents) Total() float64 {
	return r.Edge + r.Access + r.Revisit + r.Terminal + r.Direction + r.NoAction
}
