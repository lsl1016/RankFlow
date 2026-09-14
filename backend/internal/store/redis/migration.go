package redis

import (
	"context"
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"rankflow/internal/model"
	"rankflow/internal/score"
)

// MigrateLegacyBoard lazily rebuilds a v2 ranking index from the pre-v2 ZSET.
// It is idempotent and only runs when the v2 board is still empty.
func (s *Store) MigrateLegacyBoard(ctx context.Context, rankID int64, typeID string, cfg *model.RankConfig) error {
	card, err := s.rdb.ZCard(ctx, ZSetKey(rankID, typeID)).Result()
	if err != nil {
		return err
	}
	if card > 0 {
		return nil
	}
	legacy, err := s.rdb.ZRangeWithScores(ctx, LegacyZSetKey(rankID, typeID), 0, -1).Result()
	if err != nil {
		return err
	}
	if len(legacy) == 0 {
		return nil
	}
	pipe := s.rdb.TxPipeline()
	for _, z := range legacy {
		itemID := fmt.Sprint(z.Member)
		mkey := MemberKey(rankID, typeID, itemID)
		businessScore, err := s.rdb.HGet(ctx, mkey, "score").Int64()
		if err == goredis.Nil {
			continue
		}
		if err != nil {
			return err
		}
		encoded := score.LegacyEncodedMember(cfg, businessScore, z.Score, itemID)
		pipe.HSet(ctx, mkey, "member", encoded, "final", z.Score)
		pipe.HSetNX(ctx, mkey, "revision", 0)
		pipe.ZAdd(ctx, ZSetKey(rankID, typeID), goredis.Z{Score: float64(businessScore), Member: encoded})
	}
	_, err = pipe.Exec(ctx)
	return err
}

// TrimBoard applies the configured materialized leaderboard size after a bulk
// rebuild. Per-write Lua scripts enforce the same rule on the hot path.
func (s *Store) TrimBoard(ctx context.Context, rankID int64, typeID string, maxSize int, sortDesc bool) error {
	if maxSize <= 0 {
		return nil
	}
	key := ZSetKey(rankID, typeID)
	total, err := s.rdb.ZCard(ctx, key).Result()
	if err != nil || total <= int64(maxSize) {
		return err
	}
	excess := total - int64(maxSize)
	if sortDesc {
		return s.rdb.ZRemRangeByRank(ctx, key, 0, excess-1).Err()
	}
	return s.rdb.ZRemRangeByRank(ctx, key, -excess, -1).Err()
}
