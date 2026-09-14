package redis

import (
	"context"
	"os"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	addr := os.Getenv("RANKFLOW_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("RANKFLOW_TEST_REDIS_ADDR is not set")
	}
	store, err := New(addr, "", 0)
	if err != nil {
		t.Fatalf("connect test redis: %v", err)
	}
	if err := store.Client().FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("flush test redis: %v", err)
	}
	return store
}

func TestTopAndMemberRankReturnBusinessScore(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	if _, err := store.AddFinalScore(ctx, 10001, "global", "user_1", 10, 0.82, 100, true); err != nil {
		t.Fatalf("add score: %v", err)
	}

	items, err := store.Top(ctx, 10001, "global", 0, 10, true)
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if len(items) != 1 || items[0].Score != 10 {
		t.Fatalf("top must expose business score 10, got %#v", items)
	}

	rank, score, err := store.MemberRank(ctx, 10001, "global", "user_1", true)
	if err != nil {
		t.Fatalf("member rank: %v", err)
	}
	if rank != 1 || score != 10 {
		t.Fatalf("member rank must expose business score: rank=%d score=%v", rank, score)
	}
}

func TestSetFinalScoreHonorsMaxRankSize(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	cases := []struct {
		item  string
		score int64
	}{
		{item: "user_1", score: 1},
		{item: "user_2", score: 2},
		{item: "user_3", score: 3},
	}
	for _, tc := range cases {
		if err := store.SetFinalScore(ctx, 10001, "global", tc.item, tc.score, float64(tc.score)+0.1, 2, true); err != nil {
			t.Fatalf("set score for %s: %v", tc.item, err)
		}
	}

	card, err := store.Card(ctx, 10001, "global")
	if err != nil {
		t.Fatalf("card: %v", err)
	}
	if card != 2 {
		t.Fatalf("max rank size must be enforced, got card=%d", card)
	}
	rank, _, err := store.MemberRank(ctx, 10001, "global", "user_1", true)
	if err != nil {
		t.Fatalf("member rank: %v", err)
	}
	if rank != -1 {
		t.Fatalf("lowest member should be trimmed, rank=%d", rank)
	}
}

func TestAddFinalScoreHandlesLargeIntegerBusinessScore(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	const want int64 = 1_000_000_000_000
	got, err := store.AddFinalScore(ctx, 10001, "global", "large", want, 0, 100, true)
	if err != nil {
		t.Fatalf("add large score: %v", err)
	}
	if got != want {
		t.Fatalf("large business score mismatch: got=%d want=%d", got, want)
	}
}
