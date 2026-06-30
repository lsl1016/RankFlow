package score

import "rankflow/internal/model"

// maxTimestamp is an upper bound on event timestamps (≈ year 2286). Used to
// invert timestamps for the "early first" tie-break.
const maxTimestamp = 9999999999

// tsScale maps a tie-break value in [0, maxTimestamp] into a decimal fraction
// in [0, 1) so it refines ordering without disturbing the integer business
// score. Precision degrades for very large business scores, which is an
// accepted trade-off for an MVP (ties are best-effort).
const tsScale = 1e10

// IsDesc reports whether the rank sorts with higher scores first.
func IsDesc(cfg *model.RankConfig) bool {
	return cfg.SortType != model.SortScoreAsc
}

// SubDecimal returns the signed tie-break fraction to add to the business score
// when forming the Redis ZSet final_score.
//
//   - early_first: earlier events rank higher on ties
//   - late_first:  later events rank higher on ties
//   - sub_score:   business-provided secondary score ranks higher on ties
//
// For descending ranks a larger fraction wins (added); for ascending ranks the
// winner needs a smaller final score, so the fraction is subtracted.
func SubDecimal(cfg *model.RankConfig, eventTS, businessSubScore int64) float64 {
	var raw float64
	switch cfg.SameScorePolicy {
	case model.SameScoreEarlyFirst:
		v := maxTimestamp - eventTS
		if v < 0 {
			v = 0
		}
		raw = float64(v) / tsScale
	case model.SameScoreLateFirst:
		if eventTS < 0 {
			eventTS = 0
		}
		raw = float64(eventTS) / tsScale
	case model.SameScoreSubScore:
		raw = float64(businessSubScore) / tsScale
	default:
		return 0
	}
	if raw >= 1 {
		raw = 0.9999999999
	}
	if IsDesc(cfg) {
		return raw
	}
	return -raw
}

// Final computes the final_score for a given absolute business score.
func Final(cfg *model.RankConfig, businessScore, eventTS, businessSubScore int64) float64 {
	return float64(businessScore) + SubDecimal(cfg, eventTS, businessSubScore)
}
