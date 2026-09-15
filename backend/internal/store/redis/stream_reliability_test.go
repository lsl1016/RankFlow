package redis

import (
	"context"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestPersistStreamStalePendingCanBeAutoClaimed(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	if err := store.EnsurePersistGroup(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnqueuePersist(ctx, `{"hello":"world"}`); err != nil {
		t.Fatal(err)
	}

	original, err := store.ReadPersistMessage(ctx, "dead-consumer", false, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(original) != 1 {
		t.Fatalf("expected one pending message, got %#v", original)
	}

	time.Sleep(10 * time.Millisecond)
	claimed, next, err := store.AutoClaimPersistMessages(ctx, "rescuer", time.Millisecond, "0-0", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != original[0].ID {
		t.Fatalf("expected rescuer to claim %s, got %#v", original[0].ID, claimed)
	}
	if next == "" {
		t.Fatal("XAUTOCLAIM cursor must not be empty")
	}

	pending, err := store.Client().XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream:   PersistStreamKey,
		Group:    PersistGroupName,
		Start:    "-",
		End:      "+",
		Count:    10,
		Consumer: "rescuer",
	}).Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != original[0].ID {
		t.Fatalf("claimed message owner was not transferred: %#v", pending)
	}
	if err := store.AckPersist(ctx, original[0].ID); err != nil {
		t.Fatal(err)
	}
}

func TestTrimAckedPersistPreservesPendingAndUndeliveredMessages(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	if err := store.EnsurePersistGroup(ctx); err != nil {
		t.Fatal(err)
	}

	add := func(value string) string {
		t.Helper()
		id, err := store.Client().XAdd(ctx, &goredis.XAddArgs{
			Stream: PersistStreamKey,
			Values: map[string]interface{}{"payload": value},
		}).Result()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	firstID := add("first")
	secondID := add("second")
	thirdID := add("third")

	first, err := store.ReadPersistMessage(ctx, "worker-a", false, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID != firstID {
		t.Fatalf("unexpected first delivery: %#v", first)
	}
	if err := store.AckPersist(ctx, firstID); err != nil {
		t.Fatal(err)
	}

	second, err := store.ReadPersistMessage(ctx, "worker-a", false, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || second[0].ID != secondID {
		t.Fatalf("unexpected second delivery: %#v", second)
	}

	removed, err := store.TrimAckedPersist(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed < 1 {
		t.Fatalf("expected acknowledged prefix to be trimmed, removed=%d", removed)
	}

	entries, err := store.Client().XRange(ctx, PersistStreamKey, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		seen[entry.ID] = true
	}
	if seen[firstID] {
		t.Fatalf("acknowledged prefix entry %s was not trimmed", firstID)
	}
	if !seen[secondID] {
		t.Fatalf("pending entry %s was trimmed", secondID)
	}
	if !seen[thirdID] {
		t.Fatalf("undelivered entry %s was trimmed", thirdID)
	}
}
