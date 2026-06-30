package score

import (
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
