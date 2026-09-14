package score

import (
	"math"
	"testing"

	"rankflow/internal/model"
)

func TestSubDecimalEarlyFirstDesc(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreEarlyFirst}
	early := SubDecimal(cfg, 1000, 0)
	late := SubDecimal(cfg, 2000, 0)
	if !(early > late) {
		t.Fatalf("early event should outrank later on tie (desc): early=%v late=%v", early, late)
	}
	if early <= 0 || early >= 1 {
		t.Fatalf("sub decimal must be in (0,1): %v", early)
	}
}

func TestSubDecimalLateFirstDesc(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreLateFirst}
	early := SubDecimal(cfg, 1000, 0)
	late := SubDecimal(cfg, 2000, 0)
	if !(late > early) {
		t.Fatalf("later event should outrank earlier (late_first desc): early=%v late=%v", early, late)
	}
}

func TestSubDecimalAscNegates(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreAsc, SameScorePolicy: model.SameScoreEarlyFirst}
	v := SubDecimal(cfg, 1000, 0)
	if v > 0 {
		t.Fatalf("ascending tie-break should be negative, got %v", v)
	}
}

func TestFinalKeepsIntegerOrdering(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreEarlyFirst}
	low := Final(cfg, 100, 1000, 0)
	high := Final(cfg, 101, 9999, 0)
	if !(high > low) {
		t.Fatalf("higher business score must win regardless of tie-break: low=%v high=%v", low, high)
	}
}

func TestSubScoreIsMonotonicForSignedValues(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreSubScore}
	neg := SubDecimal(cfg, 0, -10)
	zero := SubDecimal(cfg, 0, 0)
	pos := SubDecimal(cfg, 0, 10)
	if !(neg < zero && zero < pos) {
		t.Fatalf("sub score mapping must be monotonic: neg=%v zero=%v pos=%v", neg, zero, pos)
	}
	if neg < 0 || pos >= 1 {
		t.Fatalf("sub score fractions must stay inside [0,1): neg=%v pos=%v", neg, pos)
	}
}

func TestNegativeSubScoreCannotCrossPrimaryScoreBoundaryDesc(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreSubScore}
	higherPrimary := Final(cfg, 100, 0, -200_000_000_000)
	lowerPrimary := Final(cfg, 99, 0, math.MaxInt64)
	if !(higherPrimary > lowerPrimary) {
		t.Fatalf("primary score must dominate sub score: score100=%v score99=%v", higherPrimary, lowerPrimary)
	}
}

func TestSubScoreCannotCrossPrimaryScoreBoundaryAsc(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreAsc, SameScorePolicy: model.SameScoreSubScore}
	lowerPrimary := Final(cfg, 99, 0, math.MinInt64)
	higherPrimary := Final(cfg, 100, 0, math.MaxInt64)
	if !(lowerPrimary < higherPrimary) {
		t.Fatalf("primary score must dominate sub score for ascending rank: score99=%v score100=%v", lowerPrimary, higherPrimary)
	}
}
