package score

import (
	"math"

	"rankflow/internal/model"
)

// maxTimestamp is an upper bound on event timestamps (approximately year 2286).
// It is used to invert timestamps for the "early first" tie-break.
const maxTimestamp int64 = 9999999999

// tsScale maps a timestamp tie-break value into [0, 1). The Redis ZSet still
// stores a float64 final score, so very large business scores can lose some
// tie-break precision. The important invariant here is that the tie-break must
// never cross an integer business-score boundary.
const tsScale = 1e10

// Keep a small gap below 1 so businessScore+tieBreak never rounds into the next
// integer score bucket at ordinary score magnitudes.
const maxTieFraction = 0.9999999999

// IsDesc reports whether the rank sorts with higher scores first.
func IsDesc(cfg *model.RankConfig) bool {
	return cfg.SortType != model.SortScoreAsc
}

// normalizeSubScore maps any signed business secondary score monotonically into
// [0, 1). Using atan avoids the old behavior where a large negative subScore
// produced a negative fraction and could let a lower primary score outrank a
// higher primary score.
func normalizeSubScore(v int64) float64 {
	raw := 0.5 + math.Atan(float64(v))/math.Pi
	if raw < 0 {
		return 0
	}
	if raw > maxTieFraction {
		return maxTieFraction
	}
	return raw
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
		raw = normalizeSubScore(businessSubScore)
	default:
		return 0
	}
	if raw < 0 {
		raw = 0
	}
	if raw > maxTieFraction {
		raw = maxTieFraction
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
