package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"rankflow/internal/model"
	"rankflow/internal/score"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	addr := os.Getenv("RANKFLOW_TEST_REDIS_ADDR")
	if addr == "" { t.Skip("RANKFLOW_TEST_REDIS_ADDR is not set") }
	store, err := New(addr, "", 0)
	if err != nil { t.Fatalf("connect test redis: %v", err) }
	if err := store.Client().FlushDB(context.Background()).Err(); err != nil { t.Fatalf("flush test redis: %v", err) }
	return store
}

func TestExactTieOrderingAtLargePrimaryScore(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreEarlyFirst}
	const primary int64 = 1_000_000_000_000

	earlyMember := score.EncodedMember(cfg, 1_700_000_000, 0, "early")
	lateMember := score.EncodedMember(cfg, 1_700_000_001, 0, "late")
	if _, _, _, err := store.AddFinalScore(ctx, 10001, "global", "early", earlyMember, primary, score.SubDecimal(cfg, 1_700_000_000, 0), 100, true); err != nil { t.Fatal(err) }
	if _, _, _, err := store.AddFinalScore(ctx, 10001, "global", "late", lateMember, primary, score.SubDecimal(cfg, 1_700_000_001, 0), 100, true); err != nil { t.Fatal(err) }

	items, err := store.Top(ctx, 10001, "global", 0, 10, true)
	if err != nil { t.Fatal(err) }
	if len(items) != 2 || items[0].ItemID != "early" || items[1].ItemID != "late" {
		t.Fatalf("expected exact one-second ordering at score 1e12, got %#v", items)
	}
	if items[0].Score != float64(primary) || items[1].Score != float64(primary) { t.Fatalf("business score changed: %#v", items) }
}

func TestSetFinalScoreHonorsMaxRankSize(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreLateFirst}
	for i, v := range []int64{1, 2, 3} {
		item := []string{"user_1", "user_2", "user_3"}[i]
		member := score.EncodedMember(cfg, int64(i+1), 0, item)
		if _, err := store.SetFinalScore(ctx, 10001, "global", item, member, v, float64(v), 2, true); err != nil { t.Fatal(err) }
	}
	card, err := store.Card(ctx, 10001, "global")
	if err != nil { t.Fatal(err) }
	if card != 2 { t.Fatalf("max rank size must be enforced, got %d", card) }
	rank, _, err := store.MemberRank(ctx, 10001, "global", "user_1", true)
	if err != nil { t.Fatal(err) }
	if rank != -1 { t.Fatalf("trimmed member should not be ranked, got %d", rank) }
}

func TestRevisionIncrementsOnWrites(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreLateFirst}
	member := score.EncodedMember(cfg, 1, 0, "user")
	_, rev1, _, err := store.AddFinalScore(ctx, 10001, "global", "user", member, 1, 0, 100, true)
	if err != nil { t.Fatal(err) }
	_, rev2, _, err := store.AddFinalScore(ctx, 10001, "global", "user", member, 1, 0, 100, true)
	if err != nil { t.Fatal(err) }
	if rev1 != 1 || rev2 != 2 { t.Fatalf("unexpected revisions: %d %d", rev1, rev2) }
}

func TestPersistStreamPendingCanBeRecovered(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	if err := store.EnsurePersistGroup(ctx); err != nil { t.Fatal(err) }
	if err := store.EnqueuePersist(ctx, `{"hello":"world"}`); err != nil { t.Fatal(err) }

	msgs, err := store.ReadPersist(ctx, "rankflow-0", false, 10*time.Millisecond)
	if err != nil { t.Fatal(err) }
	if len(msgs) != 1 { t.Fatalf("expected one new stream message, got %#v", msgs) }

	pending, err := store.ReadPersist(ctx, "rankflow-0", true, 10*time.Millisecond)
	if err != nil { t.Fatal(err) }
	if len(pending) != 1 || pending[0].ID != msgs[0].ID { t.Fatalf("expected same unacked pending message, got %#v", pending) }

	if err := store.AckPersist(ctx, msgs[0].ID); err != nil { t.Fatal(err) }
	again, err := store.ReadPersist(ctx, "rankflow-0", true, 10*time.Millisecond)
	if err != nil { t.Fatal(err) }
	if len(again) != 0 { t.Fatalf("acked message must leave pending list: %#v", again) }
}

func TestLegacyBoardMigration(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreEarlyFirst}
	if err := store.Client().HSet(ctx, MemberKey(10001, "global", "user"), "score", 10).Err(); err != nil { t.Fatal(err) }
	if err := store.Client().Do(ctx, "ZADD", LegacyZSetKey(10001, "global"), 10.8, "user").Err(); err != nil { t.Fatal(err) }
	if err := store.MigrateLegacyBoard(ctx, 10001, "global", cfg); err != nil { t.Fatal(err) }
	items, err := store.Top(ctx, 10001, "global", 0, 10, true)
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].ItemID != "user" || items[0].Score != 10 { t.Fatalf("legacy board migration failed: %#v", items) }
}
