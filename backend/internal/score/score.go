package score

import (
	"fmt"
	"math"

	"rankflow/internal/model"
)

const maxTimestamp int64 = 9999999999
const tsScale = 1e10
const maxTieFraction = 0.9999999999

// MaxExactBusinessScore is the largest integer exactly representable by a
// float64. Redis sorted-set scores use IEEE-754 doubles, so primary scores are
// kept inside this range and tie-breaking is encoded in the member string.
const MaxExactBusinessScore int64 = 1<<53 - 1

func IsDesc(cfg *model.RankConfig) bool {
	return cfg.SortType != model.SortScoreAsc
}

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

// SubDecimal is retained for persisted final_score/API compatibility. Redis
// ranking itself no longer depends on this fractional value.
func SubDecimal(cfg *model.RankConfig, eventTS, businessSubScore int64) float64 {
	var raw float64
	switch cfg.SameScorePolicy {
	case model.SameScoreEarlyFirst:
		raw = float64(maxTimestamp-clampTimestamp(eventTS)) / tsScale
	case model.SameScoreLateFirst:
		raw = float64(clampTimestamp(eventTS)) / tsScale
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

func Final(cfg *model.RankConfig, businessScore, eventTS, businessSubScore int64) float64 {
	return float64(businessScore) + SubDecimal(cfg, eventTS, businessSubScore)
}

// EncodedMember returns a fixed-width tie-order prefix followed by the item ID.
// Equal ZSET scores are ordered lexicographically by member, so this avoids
// mixing tiny tie-break decimals into a large float64 primary score.
func EncodedMember(cfg *model.RankConfig, eventTS, businessSubScore int64, itemID string) string {
	return fmt.Sprintf("%016x:%s", TieOrderKey(cfg, eventTS, businessSubScore), itemID)
}

func TieOrderKey(cfg *model.RankConfig, eventTS, businessSubScore int64) uint64 {
	var preference uint64
	switch cfg.SameScorePolicy {
	case model.SameScoreEarlyFirst:
		preference = uint64(maxTimestamp - clampTimestamp(eventTS))
	case model.SameScoreLateFirst:
		preference = uint64(clampTimestamp(eventTS))
	case model.SameScoreSubScore:
		// Flip the sign bit so unsigned order matches signed int64 order.
		preference = uint64(businessSubScore) ^ (uint64(1) << 63)
	default:
		preference = 0
	}
	if IsDesc(cfg) {
		return preference
	}
	return ^preference
}

// LegacyEncodedMember converts the old floating final_score ordering into the
// v2 member-prefix scheme during lazy migration.
func LegacyEncodedMember(cfg *model.RankConfig, businessScore int64, finalScore float64, itemID string) string {
	raw := finalScore - float64(businessScore)
	if !IsDesc(cfg) {
		raw = -raw
	}
	if raw < 0 {
		raw = 0
	}
	if raw > maxTieFraction {
		raw = maxTieFraction
	}

	var preference uint64
	switch cfg.SameScorePolicy {
	case model.SameScoreEarlyFirst, model.SameScoreLateFirst:
		preference = uint64(math.Round(raw * tsScale))
	case model.SameScoreSubScore:
		v := math.Tan((raw - 0.5) * math.Pi)
		maxI := int64(^uint64(0) >> 1)
		minI := -maxI - 1
		var recovered int64
		switch {
		case math.IsNaN(v):
			recovered = 0
		case v >= float64(maxI):
			recovered = maxI
		case v <= float64(minI):
			recovered = minI
		default:
			recovered = int64(math.Round(v))
		}
		preference = uint64(recovered) ^ (uint64(1) << 63)
	}
	if !IsDesc(cfg) {
		preference = ^preference
	}
	return fmt.Sprintf("%016x:%s", preference, itemID)
}

func clampTimestamp(ts int64) int64 {
	if ts < 0 {
		return 0
	}
	if ts > maxTimestamp {
		return maxTimestamp
	}
	return ts
}
