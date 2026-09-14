package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *redis.Client
}

func New(addr, password string, db int) (*Store, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Store{rdb: rdb}, nil
}

func (s *Store) Client() *redis.Client { return s.rdb }

func ZSetKey(rankID int64, typeID string) string {
	return fmt.Sprintf("rank:zset:%d:%s", rankID, typeID)
}

func MemberKey(rankID int64, typeID, itemID string) string {
	return fmt.Sprintf("rank:member:%d:%s:%s", rankID, typeID, itemID)
}

func ConfigKey(rankID int64) string {
	return fmt.Sprintf("rank:config:%d", rankID)
}

func IdemKey(rankID int64, requestID string) string {
	return fmt.Sprintf("rank:idem:%d:%s", rankID, requestID)
}

const PersistQueueKey = "rank:queue:persist"

func (s *Store) ClaimIdempotency(ctx context.Context, rankID int64, requestID string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, IdemKey(rankID, requestID), 1, ttl).Result()
}

func (s *Store) ReleaseIdempotency(ctx context.Context, rankID int64, requestID string) {
	s.rdb.Del(ctx, IdemKey(rankID, requestID))
}

var addScoreScript = redis.NewScript(`
local zkey = KEYS[1]
local mkey = KEYS[2]
local member = ARGV[1]
local delta = tonumber(ARGV[2])
local subdecimal = tonumber(ARGV[3])
local maxsize = tonumber(ARGV[4])
local sortDesc = ARGV[5]

local cur = tonumber(redis.call('HGET', mkey, 'score') or '0')
local newScore = cur + delta
redis.call('HSET', mkey, 'score', newScore)

local finalScore = newScore + subdecimal
redis.call('ZADD', zkey, finalScore, member)

if maxsize > 0 then
  local total = redis.call('ZCARD', zkey)
  if total > maxsize then
    local excess = total - maxsize
    if sortDesc == '1' then
      redis.call('ZREMRANGEBYRANK', zkey, 0, excess - 1)
    else
      redis.call('ZREMRANGEBYRANK', zkey, -excess, -1)
    end
  end
end
return newScore
`)

func (s *Store) AddFinalScore(ctx context.Context, rankID int64, typeID, itemID string, delta int64, subDecimal float64, maxSize int, sortDesc bool) (int64, error) {
	descFlag := "0"
	if sortDesc {
		descFlag = "1"
	}
	return addScoreScript.Run(ctx, s.rdb,
		[]string{ZSetKey(rankID, typeID), MemberKey(rankID, typeID, itemID)},
		itemID, delta, subDecimal, maxSize, descFlag,
	).Int64()
}

var setScoreScript = redis.NewScript(`
local zkey = KEYS[1]
local mkey = KEYS[2]
local member = ARGV[1]
local businessScore = ARGV[2]
local finalScore = tonumber(ARGV[3])
local maxsize = tonumber(ARGV[4])
local sortDesc = ARGV[5]

redis.call('HSET', mkey, 'score', businessScore)
redis.call('ZADD', zkey, finalScore, member)

if maxsize > 0 then
  local total = redis.call('ZCARD', zkey)
  if total > maxsize then
    local excess = total - maxsize
    if sortDesc == '1' then
      redis.call('ZREMRANGEBYRANK', zkey, 0, excess - 1)
    else
      redis.call('ZREMRANGEBYRANK', zkey, -excess, -1)
    end
  end
end
return 1
`)

func (s *Store) SetFinalScore(ctx context.Context, rankID int64, typeID, itemID string, businessScore int64, finalScore float64, maxSize int, sortDesc bool) error {
	descFlag := "0"
	if sortDesc {
		descFlag = "1"
	}
	return setScoreScript.Run(ctx, s.rdb,
		[]string{ZSetKey(rankID, typeID), MemberKey(rankID, typeID, itemID)},
		itemID, businessScore, finalScore, maxSize, descFlag,
	).Err()
}

func (s *Store) GetScore(ctx context.Context, rankID int64, typeID, itemID string) (int64, error) {
	v, err := s.rdb.HGet(ctx, MemberKey(rankID, typeID, itemID), "score").Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

func (s *Store) GetFinalScore(ctx context.Context, rankID int64, typeID, itemID string) (float64, error) {
	v, err := s.rdb.ZScore(ctx, ZSetKey(rankID, typeID), itemID).Result()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

type RankEntry struct {
	Rank   int     `json:"rank"`
	ItemID string  `json:"itemId"`
	Score  float64 `json:"score"`
}

func (s *Store) Top(ctx context.Context, rankID int64, typeID string, offset, limit int, sortDesc bool) ([]RankEntry, error) {
	key := ZSetKey(rankID, typeID)
	var zs []redis.Z
	var err error
	if sortDesc {
		zs, err = s.rdb.ZRevRangeWithScores(ctx, key, int64(offset), int64(offset+limit-1)).Result()
	} else {
		zs, err = s.rdb.ZRangeWithScores(ctx, key, int64(offset), int64(offset+limit-1)).Result()
	}
	if err != nil {
		return nil, err
	}
	if len(zs) == 0 {
		return []RankEntry{}, nil
	}

	pipe := s.rdb.Pipeline()
	itemIDs := make([]string, len(zs))
	scoreCmds := make([]*redis.StringCmd, len(zs))
	for i, z := range zs {
		itemIDs[i] = fmt.Sprint(z.Member)
		scoreCmds[i] = pipe.HGet(ctx, MemberKey(rankID, typeID, itemIDs[i]), "score")
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	out := make([]RankEntry, 0, len(zs))
	for i := range zs {
		businessScore, err := scoreCmds[i].Int64()
		if err != nil {
			return nil, fmt.Errorf("load business score for item %q: %w", itemIDs[i], err)
		}
		out = append(out, RankEntry{
			Rank:   offset + i + 1,
			ItemID: itemIDs[i],
			Score:  float64(businessScore),
		})
	}
	return out, nil
}

func (s *Store) MemberRank(ctx context.Context, rankID int64, typeID, itemID string, sortDesc bool) (int, float64, error) {
	key := ZSetKey(rankID, typeID)
	var rank int64
	var err error
	if sortDesc {
		rank, err = s.rdb.ZRevRank(ctx, key, itemID).Result()
	} else {
		rank, err = s.rdb.ZRank(ctx, key, itemID).Result()
	}
	if err == redis.Nil {
		return -1, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	businessScore, err := s.GetScore(ctx, rankID, typeID, itemID)
	if err != nil {
		return 0, 0, err
	}
	return int(rank) + 1, float64(businessScore), nil
}

func (s *Store) Around(ctx context.Context, rankID int64, typeID, itemID string, before, after int, sortDesc bool) ([]RankEntry, error) {
	rank, _, err := s.MemberRank(ctx, rankID, typeID, itemID, sortDesc)
	if err != nil {
		return nil, err
	}
	if rank < 0 {
		return []RankEntry{}, nil
	}
	start := rank - 1 - before
	if start < 0 {
		start = 0
	}
	limit := before + after + 1
	return s.Top(ctx, rankID, typeID, start, limit, sortDesc)
}

func (s *Store) Card(ctx context.Context, rankID int64, typeID string) (int64, error) {
	return s.rdb.ZCard(ctx, ZSetKey(rankID, typeID)).Result()
}

func (s *Store) SetConfigCache(ctx context.Context, rankID int64, payload string, ttl time.Duration) error {
	return s.rdb.Set(ctx, ConfigKey(rankID), payload, ttl).Err()
}

func (s *Store) GetConfigCache(ctx context.Context, rankID int64) (string, bool, error) {
	v, err := s.rdb.Get(ctx, ConfigKey(rankID)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) DelConfigCache(ctx context.Context, rankID int64) error {
	return s.rdb.Del(ctx, ConfigKey(rankID)).Err()
}

func (s *Store) EnqueuePersist(ctx context.Context, payload string) error {
	return s.rdb.LPush(ctx, PersistQueueKey, payload).Err()
}

func (s *Store) DequeuePersist(ctx context.Context, timeout time.Duration) (string, error) {
	res, err := s.rdb.BRPop(ctx, timeout, PersistQueueKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(res) == 2 {
		return res[1], nil
	}
	return "", nil
}
