package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func SubBoardStatusKey(rankID int64, typeID string) string {
	return fmt.Sprintf("rank:subboard:status:%d:%s", rankID, typeID)
}

func (s *Store) GetSubBoardStatusCache(ctx context.Context, rankID int64, typeID string) (int, bool, error) {
	status, err := s.rdb.Get(ctx, SubBoardStatusKey(rankID, typeID)).Int()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return status, true, nil
}

func (s *Store) SetSubBoardStatusCache(ctx context.Context, rankID int64, typeID string, status int, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return s.rdb.Set(ctx, SubBoardStatusKey(rankID, typeID), status, ttl).Err()
}

func (s *Store) DelSubBoardStatusCache(ctx context.Context, rankID int64, typeID string) error {
	return s.rdb.Del(ctx, SubBoardStatusKey(rankID, typeID)).Err()
}
