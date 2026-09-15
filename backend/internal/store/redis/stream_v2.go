package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// PersistStreamMessage supports both the legacy JSON payload field and the v2
// structured fields emitted by the atomic score-write Lua script.
type PersistStreamMessage struct {
	ID      string
	Payload string
	Values  map[string]string
}

func (s *Store) ReadPersistMessage(ctx context.Context, consumer string, pending bool, block time.Duration) ([]PersistStreamMessage, error) {
	start := ">"
	if pending {
		start = "0"
	}
	streams, err := s.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    PersistGroupName,
		Consumer: consumer,
		Streams:  []string{PersistStreamKey, start},
		Count:    1,
		Block:    block,
	}).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	out := make([]PersistStreamMessage, 0, 1)
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			out = append(out, persistStreamMessageFromRedis(msg))
		}
	}
	return out, nil
}
