package mysql

import (
	"context"
	"os"
	"testing"
	"time"

	"rankflow/internal/model"
)

func testMySQLStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("RANKFLOW_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("RANKFLOW_TEST_MYSQL_DSN is not set")
	}
	store, err := New(dsn)
	if err != nil {
		t.Fatalf("connect test mysql: %v", err)
	}
	if err := store.AutoMigrate(); err != nil {
		t.Fatalf("auto migrate test mysql: %v", err)
	}
	return store
}

func TestUpdateRankPreservesScoreIntegerDigits(t *testing.T) {
	store := testMySQLStore(t)
	ctx := context.Background()
	rankID := time.Now().UnixNano()
	now := time.Now()

	cfg := &model.RankConfig{
		RankID:             rankID,
		RankName:           "before",
		BizCode:            "test",
		TargetType:         "user",
		Status:             model.StatusDraft,
		SortType:           model.SortScoreDesc,
		SameScorePolicy:    model.SameScoreEarlyFirst,
		ScoreIntegerDigits: 12,
		MaxRankSize:        100,
		CacheTTLSeconds:    60,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	tc := &model.RankTimeConfig{
		TimeType:   model.TimeNone,
		Timezone:   "Asia/Shanghai",
		AnchorType: model.AnchorEventTime,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := store.CreateRank(ctx, cfg, nil, tc); err != nil {
		t.Fatalf("create rank: %v", err)
	}
	t.Cleanup(func() {
		store.DB().Where("rank_id = ?", rankID).Delete(&model.RankDimensionConfig{})
		store.DB().Where("rank_id = ?", rankID).Delete(&model.RankTimeConfig{})
		store.DB().Where("rank_id = ?", rankID).Delete(&model.RankConfig{})
	})

	update := &model.RankConfig{
		RankID:          rankID,
		RankName:        "after",
		BizCode:         "test",
		TargetType:      "user",
		SortType:        model.SortScoreAsc,
		SameScorePolicy: model.SameScoreLateFirst,
		MaxRankSize:     200,
		CacheTTLSeconds: 120,
		UpdatedAt:       time.Now(),
	}
	updatedTime := &model.RankTimeConfig{
		TimeType:   model.TimeDay,
		Timezone:   "Asia/Shanghai",
		AnchorType: model.AnchorEventTime,
		UpdatedAt:  time.Now(),
	}
	if err := store.UpdateRank(ctx, update, nil, updatedTime); err != nil {
		t.Fatalf("update rank: %v", err)
	}

	got, err := store.GetRank(ctx, rankID)
	if err != nil {
		t.Fatalf("get rank: %v", err)
	}
	if got.ScoreIntegerDigits != 12 {
		t.Fatalf("score_integer_digits must be preserved, got=%d want=12", got.ScoreIntegerDigits)
	}
	if got.RankName != "after" || got.SortType != model.SortScoreAsc {
		t.Fatalf("expected editable fields to update, got=%+v", got)
	}
}
