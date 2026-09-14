package redis

import (
	"context"
	"errors"
	"testing"
	"time"
)

func atomicRequest() AtomicWriteRequest {
	return AtomicWriteRequest{
		Mode:           WriteModeAdd,
		RankID:         10001,
		TypeID:         "global",
		ItemID:         "user_1",
		EncodedMember:  "0000000000000001:user_1",
		Value:          10,
		SubScore:       0,
		EventTime:      1_700_000_000,
		SubDecimal:     0.8,
		MaxSize:        100,
		SortDesc:       true,
		RequestID:      "req-1",
		Fingerprint:    "fingerprint-1",
		IdempotencyTTL: time.Hour,
		TraceID:        "trace-1",
	}
}

func TestAtomicWriteIsIdempotentAndEnqueuesOnce(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	req := atomicRequest()

	first, err := store.WriteScoreAtomic(ctx, req)
	if err != nil {
		t.Fatalf("first atomic write: %v", err)
	}
	if !first.Applied || first.Duplicate || first.Score != 10 || first.Revision != 1 {
		t.Fatalf("unexpected first result: %#v", first)
	}

	second, err := store.WriteScoreAtomic(ctx, req)
	if err != nil {
		t.Fatalf("duplicate atomic write: %v", err)
	}
	if !second.Duplicate || second.Applied || second.Score != 10 || second.Revision != 1 {
		t.Fatalf("unexpected duplicate result: %#v", second)
	}

	score, err := store.GetScore(ctx, req.RankID, req.TypeID, req.ItemID)
	if err != nil {
		t.Fatalf("get score: %v", err)
	}
	if score != 10 {
		t.Fatalf("score applied more than once: got=%d want=10", score)
	}

	streamLen, err := store.Client().XLen(ctx, PersistStreamKey).Result()
	if err != nil {
		t.Fatalf("stream length: %v", err)
	}
	if streamLen != 1 {
		t.Fatalf("persist stream must contain one message, got=%d", streamLen)
	}

	record, found, err := store.GetIdempotencyRecord(ctx, req.RankID, req.RequestID)
	if err != nil {
		t.Fatalf("get idempotency record: %v", err)
	}
	if !found || record.Score != 10 || record.Revision != 1 || record.Fingerprint != req.Fingerprint {
		t.Fatalf("unexpected idempotency record: found=%v record=%#v", found, record)
	}
}

func TestAtomicWriteRejectsRequestIDReuseForDifferentPayload(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	req := atomicRequest()
	if _, err := store.WriteScoreAtomic(ctx, req); err != nil {
		t.Fatalf("first atomic write: %v", err)
	}

	reused := req
	reused.Value = 99
	reused.Fingerprint = "fingerprint-2"
	result, err := store.WriteScoreAtomic(ctx, reused)
	if err != nil {
		t.Fatalf("request id reuse: %v", err)
	}
	if !result.Conflict {
		t.Fatalf("expected idempotency conflict, got %#v", result)
	}
	score, err := store.GetScore(ctx, req.RankID, req.TypeID, req.ItemID)
	if err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("conflicting retry mutated score: got=%d want=10", score)
	}
	streamLen, err := store.Client().XLen(ctx, PersistStreamKey).Result()
	if err != nil {
		t.Fatal(err)
	}
	if streamLen != 1 {
		t.Fatalf("conflicting retry enqueued a message: len=%d", streamLen)
	}
}

func TestAtomicWritePreflightPreventsPartialMutationWhenStreamKeyInvalid(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	if err := store.Client().Set(ctx, PersistStreamKey, "not-a-stream", 0).Err(); err != nil {
		t.Fatal(err)
	}
	req := atomicRequest()
	if _, err := store.WriteScoreAtomic(ctx, req); err == nil {
		t.Fatal("expected atomic write to reject incompatible stream key")
	}
	score, err := store.GetScore(ctx, req.RankID, req.TypeID, req.ItemID)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("failed atomic write mutated score: got=%d want=0", score)
	}
	_, found, err := store.GetIdempotencyRecord(ctx, req.RankID, req.RequestID)
	if err != nil && !errors.Is(err, ErrLegacyIdempotencyRecord) {
		t.Fatal(err)
	}
	if found {
		t.Fatal("failed atomic write must not claim idempotency key")
	}
}
