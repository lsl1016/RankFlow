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

func TestScoreWriteCachesSubBoardStatusAndAvoidsRepeatedUpsert(t *testing.T) {
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
		RankName:           "write-hot-path",
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
	first, err := svc.AddScore(ctx, rankID, &AddScoreInput{
		RequestID:  "hot-path-1",
		ItemID:     "user_1",
		Score:      10,
		EventTime:  now.Unix(),
		Dimensions: map[string]string{},
	})
	if err != nil {
		t.Fatalf("first score write: %v", err)
	}
	if first.TypeID != "global" {
		t.Fatalf("unexpected type id: %s", first.TypeID)
	}

	before, err := my.GetSubBoard(ctx, rankID, "global")
	if err != nil {
		t.Fatal(err)
	}
	status, found, err := rd.GetSubBoardStatusCache(ctx, rankID, "global")
	if err != nil {
		t.Fatal(err)
	}
	if !found || status != model.StatusOnline {
		t.Fatalf("subboard status cache not populated: found=%v status=%d", found, status)
	}

	time.Sleep(25 * time.Millisecond)
	if _, err := svc.AddScore(ctx, rankID, &AddScoreInput{
		RequestID:  "hot-path-2",
		ItemID:     "user_1",
		Score:      5,
		EventTime:  now.Unix() + 1,
		Dimensions: map[string]string{},
	}); err != nil {
		t.Fatalf("second score write: %v", err)
	}
	after, err := my.GetSubBoard(ctx, rankID, "global")
	if err != nil {
		t.Fatal(err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("hot-path write unexpectedly upserted subboard metadata: before=%s after=%s", before.UpdatedAt, after.UpdatedAt)
	}

	if err := svc.SetSubBoardStatus(ctx, rankID, "global", model.StatusOffline); err != nil {
		t.Fatalf("set subboard offline: %v", err)
	}
	_, err = svc.AddScore(ctx, rankID, &AddScoreInput{
		RequestID:  "hot-path-3",
		ItemID:     "user_1",
		Score:      1,
		EventTime:  now.Unix() + 2,
		Dimensions: map[string]string{},
	})
	if !errors.Is(err, ErrNotOnline) {
		t.Fatalf("offline subboard must reject writes, got %v", err)
	}
}
