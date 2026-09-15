package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	// DefaultPersistClaimMinIdle is long enough to avoid stealing healthy work,
	// while still recovering jobs quickly after a worker disappears.
	DefaultPersistClaimMinIdle = 30 * time.Second
	DefaultPersistClaimBatch   = int64(32)
)

func persistStreamMessageFromRedis(msg goredis.XMessage) PersistStreamMessage {
	item := PersistStreamMessage{ID: msg.ID, Values: map[string]string{}}
	for key, value := range msg.Values {
		item.Values[key] = fmt.Sprint(value)
	}
	item.Payload = item.Values["payload"]
	return item
}

// AutoClaimPersistMessages transfers stale pending entries from dead or stalled
// consumers to consumer. The returned cursor should be passed back on the next
// scan so large pending lists are traversed without repeatedly starting over.
func (s *Store) AutoClaimPersistMessages(ctx context.Context, consumer string, minIdle time.Duration, start string, count int64) ([]PersistStreamMessage, string, error) {
	if consumer == "" {
		return nil, start, fmt.Errorf("persist stream consumer is required")
	}
	if start == "" {
		start = "0-0"
	}
	if count <= 0 {
		count = DefaultPersistClaimBatch
	}
	messages, next, err := s.rdb.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
		Stream:   PersistStreamKey,
		Group:    PersistGroupName,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    start,
		Count:    count,
	}).Result()
	if err == goredis.Nil {
		return nil, next, nil
	}
	if err != nil {
		return nil, start, err
	}
	out := make([]PersistStreamMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, persistStreamMessageFromRedis(msg))
	}
	return out, next, nil
}

// TrimAckedPersist removes only the stream prefix that is known to be safe.
//
// If pending entries exist, the oldest pending ID is the trim boundary, so no
// retryable message can be removed. If there are no pending entries, the
// consumer group's last-delivered ID is the boundary; entries newer than it may
// still be undispatched and are therefore preserved.
func (s *Store) TrimAckedPersist(ctx context.Context) (int64, error) {
	pending, err := s.rdb.XPending(ctx, PersistStreamKey, PersistGroupName).Result()
	if err != nil {
		return 0, err
	}

	cutoff := ""
	if pending.Count > 0 {
		cutoff = pending.Lower
	} else {
		groups, err := s.rdb.XInfoGroups(ctx, PersistStreamKey).Result()
		if err != nil {
			return 0, err
		}
		found := false
		for _, group := range groups {
			if group.Name == PersistGroupName {
				cutoff = group.LastDeliveredID
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("persist stream group %q not found", PersistGroupName)
		}
	}

	if cutoff == "" || cutoff == "0-0" {
		return 0, nil
	}
	return s.rdb.XTrimMinID(ctx, PersistStreamKey, cutoff).Result()
}
