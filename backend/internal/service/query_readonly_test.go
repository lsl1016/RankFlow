package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	mysqlstore "rankflow/internal/store/mysql"
	redisstore "rankflow/internal/store/redis"
)

func TestQueryTopDoesNotMaterializeMissingSubBoard(t *testing.T) {
	dsn := os.Getenv("RANKFLOW_TEST_MYSQL_DSN")
	redisAddr := os.Getenv("RANKFLOW_TEST_REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		t.Skip("integration database environment is not set")
	}

	ctx := context.Background()
	my, err := mysqlstore.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := my.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	rd, err := redisstore.New(redisAddr, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := rd.Client().FlushDB(ctx).Err(); err != nil {
		t.Fatal(err)
	}

	rankID := time.Now().UnixNano()
	now := time.Now()
	cfg := &model.RankConfig{
		RankID:             rankID,
		RankName:           "read-only-query",
		BizCode:            "test",
		TargetType:         "user",
		Status:             model.StatusOnline,
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
	if err := my.CreateRank(ctx, cfg, nil, tc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankMemberScore{})
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankSubBoard{})
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankTimeConfig{})
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankConfig{})
	})

	svc := New(my, rd, zap.NewNop(), nil)
	result, err := svc.QueryTop(ctx, rankID, nil, 0, 0, 10)
	if err != nil {
		t.Fatalf("query top: %v", err)
	}
	if result.TypeID != "global" || result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("unexpected empty query result: %#v", result)
	}
	if _, err := my.GetSubBoard(ctx, rankID, "global"); !errors.Is(err, mysqlstore.ErrNotFound) {
		t.Fatalf("query materialized rank_sub_board, err=%v", err)
	}
}
