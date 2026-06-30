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

// --- key builders (see design doc section 9) ---

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

// --- idempotency ---

// ClaimIdempotency atomically marks a request as processed. Returns true when
// the caller won the claim (i.e. it is the first time we see this requestID).
func (s *Store) ClaimIdempotency(ctx context.Context, rankID int64, requestID string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, IdemKey(rankID, requestID), 1, ttl).Result()
}

// ReleaseIdempotency removes the idempotency marker (used on rollback).
func (s *Store) ReleaseIdempotency(ctx context.Context, rankID int64, requestID string) {
	s.rdb.Del(ctx, IdemKey(rankID, requestID))
}

// --- zset operations ---

// AddFinalScore atomically applies a score delta to the member and stores the
// resulting final_score back into the zset with the tie-break encoding handled
// by the caller. It returns the new business score (integer part).
//
// The script keeps the running integer business score in a hash field so the
// final_score (which embeds tie-break decimals) can be recomputed consistently.
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
return tostring(newScore)
`)

func (s *Store) AddFinalScore(ctx context.Context, rankID int64, typeID, itemID string, delta int64, subDecimal float64, maxSize int, sortDesc bool) (int64, error) {
	descFlag := "0"
	if sortDesc {
		descFlag = "1"
	}
	res, err := addScoreScript.Run(ctx, s.rdb,
		[]string{ZSetKey(rankID, typeID), MemberKey(rankID, typeID, itemID)},
		itemID, delta, subDecimal, maxSize, descFlag,
	).Text()
	if err != nil {
		return 0, err
	}
	var newScore int64
	if _, err := fmt.Sscanf(res, "%d", &newScore); err != nil {
		return 0, err
	}
	return newScore, nil
}

// SetFinalScore overwrites a member's business score and final_score.
func (s *Store) SetFinalScore(ctx context.Context, rankID int64, typeID, itemID string, score int64, finalScore float64) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, MemberKey(rankID, typeID, itemID), "score", score)
	pipe.ZAdd(ctx, ZSetKey(rankID, typeID), redis.Z{Score: finalScore, Member: itemID})
	_, err := pipe.Exec(ctx)
	return err
}

// GetScore returns the stored integer business score for a member (0 if absent).
func (s *Store) GetScore(ctx context.Context, rankID int64, typeID, itemID string) (int64, error) {
	v, err := s.rdb.HGet(ctx, MemberKey(rankID, typeID, itemID), "score").Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return v, err
}

// RankEntry is one row of a leaderboard query.
type RankEntry struct {
	Rank   int     `json:"rank"`
	ItemID string  `json:"itemId"`
	Score  float64 `json:"score"`
}

// Top returns members ordered according to sortDesc with their 1-based rank.
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
	out := make([]RankEntry, 0, len(zs))
	for i, z := range zs {
		out = append(out, RankEntry{
			Rank:   offset + i + 1,
			ItemID: fmt.Sprint(z.Member),
			Score:  truncateScore(z.Score),
		})
	}
	return out, nil
}

// MemberRank returns the 1-based rank and final score of a member.
// rank is -1 when the member is not present.
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
	score, err := s.rdb.ZScore(ctx, key, itemID).Result()
	if err == redis.Nil {
		return -1, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return int(rank) + 1, truncateScore(score), nil
}

// Around returns members surrounding the given member (before/after window).
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

// --- config cache ---

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

// --- async persist queue ---

func (s *Store) EnqueuePersist(ctx context.Context, payload string) error {
	return s.rdb.LPush(ctx, PersistQueueKey, payload).Err()
}

// DequeuePersist blocks up to timeout waiting for a persist job. Returns
// ("", nil) on timeout.
func (s *Store) DequeuePersist(ctx context.Context, timeout time.Duration) (string, error) {
	res, err := s.rdb.BRPop(ctx, timeout, PersistQueueKey).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// res = [key, value]
	if len(res) == 2 {
		return res[1], nil
	}
	return "", nil
}

// truncateScore drops the tie-break decimal noise for display, leaving the
// business-relevant magnitude. We keep two decimals so callers that genuinely
// use sub_score decimals still see them.
func truncateScore(f float64) float64 {
	return float64(int64(f*100)) / 100
}
