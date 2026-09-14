package service

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/model"
	mysqlstore "rankflow/internal/store/mysql"
	redisstore "rankflow/internal/store/redis"
)

func TestPrepareBoardRebuildsRedisFromMySQL(t *testing.T) {
	dsn := os.Getenv("RANKFLOW_TEST_MYSQL_DSN")
	redisAddr := os.Getenv("RANKFLOW_TEST_REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		t.Skip("integration database environment is not set")
	}
	ctx := context.Background()
	my, err := mysqlstore.New(dsn)
	if err != nil { t.Fatal(err) }
	if err := my.AutoMigrate(); err != nil { t.Fatal(err) }
	rd, err := redisstore.New(redisAddr, "", 0)
	if err != nil { t.Fatal(err) }
	if err := rd.Client().FlushDB(ctx).Err(); err != nil { t.Fatal(err) }

	rankID := time.Now().UnixNano()
	now := time.Now()
	cfg := &model.RankConfig{
		RankID: rankID, RankName: "rebuild", BizCode: "test", TargetType: "user",
		Status: model.StatusOnline, SortType: model.SortScoreDesc,
		SameScorePolicy: model.SameScoreEarlyFirst, ScoreIntegerDigits: 12,
		MaxRankSize: 100, CacheTTLSeconds: 60, CreatedAt: now, UpdatedAt: now,
	}
	tc := &model.RankTimeConfig{TimeType: model.TimeNone, Timezone: "Asia/Shanghai", AnchorType: model.AnchorEventTime, CreatedAt: now, UpdatedAt: now}
	if err := my.CreateRank(ctx, cfg, nil, tc); err != nil { t.Fatal(err) }
	t.Cleanup(func() {
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankMemberScore{})
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankTimeConfig{})
		my.DB().Where("rank_id = ?", rankID).Delete(&model.RankConfig{})
	})

	early := now.Add(-time.Second)
	rows := []*model.RankMemberScore{
		{RankID: rankID, TypeID: "global", ItemID: "early", Score: 1_000_000_000_000, FinalScore: 1_000_000_000_000.8, Revision: 3, LastEventTime: &early, CreatedAt: now, UpdatedAt: now},
		{RankID: rankID, TypeID: "global", ItemID: "late", Score: 1_000_000_000_000, FinalScore: 1_000_000_000_000.8, Revision: 2, LastEventTime: &now, CreatedAt: now, UpdatedAt: now},
	}
	for _, row := range rows {
		if err := my.UpsertMemberScoreIfNewer(ctx, row); err != nil { t.Fatal(err) }
	}

	svc := New(my, rd, zap.NewNop(), nil)
	rc := &ResolvedConfig{Config: *cfg, Time: *tc}
	if err := svc.prepareBoard(ctx, rc, rankID, "global"); err != nil { t.Fatal(err) }
	items, err := rd.Top(ctx, rankID, "global", 0, 10, true)
	if err != nil { t.Fatal(err) }
	if len(items) != 2 || items[0].ItemID != "early" || items[1].ItemID != "late" {
		t.Fatalf("unexpected rebuilt ordering: %#v", items)
	}
	rev, err := rd.GetRevision(ctx, rankID, "global", "early")
	if err != nil { t.Fatal(err) }
	if rev != 3 { t.Fatalf("revision not restored: got %d want 3", rev) }
}
