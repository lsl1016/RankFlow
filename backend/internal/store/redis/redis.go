package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *goredis.Client
}

func New(addr, password string, db int) (*Store, error) {
	rdb := goredis.NewClient(&goredis.Options{Addr: addr, Password: password, DB: db})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Store{rdb: rdb}, nil
}

func (s *Store) Client() *goredis.Client { return s.rdb }

func ZSetKey(rankID int64, typeID string) string {
	return fmt.Sprintf("rank:zset:v2:%d:%s", rankID, typeID)
}

func LegacyZSetKey(rankID int64, typeID string) string {
	return fmt.Sprintf("rank:zset:%d:%s", rankID, typeID)
}

func MemberKey(rankID int64, typeID, itemID string) string {
	return fmt.Sprintf("rank:member:%d:%s:%s", rankID, typeID, itemID)
}

func ConfigKey(rankID int64) string { return fmt.Sprintf("rank:config:%d", rankID) }
func IdemKey(rankID int64, requestID string) string { return fmt.Sprintf("rank:idem:%d:%s", rankID, requestID) }

const (
	PersistStreamKey   = "rank:stream:persist"
	PersistGroupName   = "rankflow-persist"
	legacyPersistQueue = "rank:queue:persist"
)

func (s *Store) ClaimIdempotency(ctx context.Context, rankID int64, requestID string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, IdemKey(rankID, requestID), 1, ttl).Result()
}

func (s *Store) ReleaseIdempotency(ctx context.Context, rankID int64, requestID string) {
	_ = s.rdb.Del(ctx, IdemKey(rankID, requestID)).Err()
}

var addScoreScript = goredis.NewScript(`
local zkey = KEYS[1]
local mkey = KEYS[2]
local member = ARGV[1]
local delta = tonumber(ARGV[2])
local maxsize = tonumber(ARGV[3])
local sortDesc = ARGV[4]
local subdecimal = tonumber(ARGV[5])

local oldMember = redis.call('HGET', mkey, 'member')
if oldMember then
  redis.call('ZREM', zkey, oldMember)
end
local cur = tonumber(redis.call('HGET', mkey, 'score') or '0')
local newScore = cur + delta
local revision = redis.call('HINCRBY', mkey, 'revision', 1)
local finalScore = newScore + subdecimal
redis.call('HSET', mkey, 'score', newScore, 'member', member, 'final', finalScore)
redis.call('ZADD', zkey, newScore, member)

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
return {tostring(newScore), tostring(revision), tostring(finalScore)}
`)

func (s *Store) AddFinalScore(ctx context.Context, rankID int64, typeID, itemID, encodedMember string, delta int64, subDecimal float64, maxSize int, sortDesc bool) (int64, int64, float64, error) {
	descFlag := "0"
	if sortDesc { descFlag = "1" }
	res, err := addScoreScript.Run(ctx, s.rdb,
		[]string{ZSetKey(rankID, typeID), MemberKey(rankID, typeID, itemID)},
		encodedMember, delta, maxSize, descFlag, subDecimal,
	).Result()
	if err != nil { return 0, 0, 0, err }
	vals, ok := res.([]interface{})
	if !ok || len(vals) != 3 { return 0, 0, 0, fmt.Errorf("unexpected add score result: %#v", res) }
	var score, revision int64
	var final float64
	if _, err := fmt.Sscan(fmt.Sprint(vals[0]), &score); err != nil { return 0, 0, 0, err }
	if _, err := fmt.Sscan(fmt.Sprint(vals[1]), &revision); err != nil { return 0, 0, 0, err }
	if _, err := fmt.Sscan(fmt.Sprint(vals[2]), &final); err != nil { return 0, 0, 0, err }
	return score, revision, final, nil
}

var setScoreScript = goredis.NewScript(`
local zkey = KEYS[1]
local mkey = KEYS[2]
local member = ARGV[1]
local businessScore = tonumber(ARGV[2])
local finalScore = tonumber(ARGV[3])
local maxsize = tonumber(ARGV[4])
local sortDesc = ARGV[5]

local oldMember = redis.call('HGET', mkey, 'member')
if oldMember then
  redis.call('ZREM', zkey, oldMember)
end
local revision = redis.call('HINCRBY', mkey, 'revision', 1)
redis.call('HSET', mkey, 'score', businessScore, 'member', member, 'final', finalScore)
redis.call('ZADD', zkey, businessScore, member)

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
return tostring(revision)
`)

func (s *Store) SetFinalScore(ctx context.Context, rankID int64, typeID, itemID, encodedMember string, businessScore int64, finalScore float64, maxSize int, sortDesc bool) (int64, error) {
	descFlag := "0"
	if sortDesc { descFlag = "1" }
	return setScoreScript.Run(ctx, s.rdb,
		[]string{ZSetKey(rankID, typeID), MemberKey(rankID, typeID, itemID)},
		encodedMember, businessScore, finalScore, maxSize, descFlag,
	).Int64()
}

func (s *Store) RestoreMember(ctx context.Context, rankID int64, typeID, itemID, encodedMember string, businessScore, revision int64, finalScore float64) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, MemberKey(rankID, typeID, itemID), "score", businessScore, "revision", revision, "member", encodedMember, "final", finalScore)
	pipe.ZAdd(ctx, ZSetKey(rankID, typeID), goredis.Z{Score: float64(businessScore), Member: encodedMember})
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) GetScore(ctx context.Context, rankID int64, typeID, itemID string) (int64, error) {
	v, err := s.rdb.HGet(ctx, MemberKey(rankID, typeID, itemID), "score").Int64()
	if err == goredis.Nil { return 0, nil }
	return v, err
}

func (s *Store) GetRevision(ctx context.Context, rankID int64, typeID, itemID string) (int64, error) {
	v, err := s.rdb.HGet(ctx, MemberKey(rankID, typeID, itemID), "revision").Int64()
	if err == goredis.Nil { return 0, nil }
	return v, err
}

func (s *Store) GetFinalScore(ctx context.Context, rankID int64, typeID, itemID string) (float64, error) {
	v, err := s.rdb.HGet(ctx, MemberKey(rankID, typeID, itemID), "final").Float64()
	if err == goredis.Nil { return 0, nil }
	return v, err
}

type RankEntry struct {
	Rank   int     `json:"rank"`
	ItemID string  `json:"itemId"`
	Score  float64 `json:"score"`
}

func decodeItemID(encoded string) (string, error) {
	if len(encoded) < 18 || encoded[16] != ':' { return "", fmt.Errorf("invalid encoded member %q", encoded) }
	return encoded[17:], nil
}

func (s *Store) Top(ctx context.Context, rankID int64, typeID string, offset, limit int, sortDesc bool) ([]RankEntry, error) {
	key := ZSetKey(rankID, typeID)
	var zs []goredis.Z
	var err error
	if sortDesc { zs, err = s.rdb.ZRevRangeWithScores(ctx, key, int64(offset), int64(offset+limit-1)).Result() } else { zs, err = s.rdb.ZRangeWithScores(ctx, key, int64(offset), int64(offset+limit-1)).Result() }
	if err != nil { return nil, err }
	out := make([]RankEntry, 0, len(zs))
	for i, z := range zs {
		itemID, err := decodeItemID(fmt.Sprint(z.Member))
		if err != nil { return nil, err }
		out = append(out, RankEntry{Rank: offset+i+1, ItemID: itemID, Score: z.Score})
	}
	return out, nil
}

func (s *Store) MemberRank(ctx context.Context, rankID int64, typeID, itemID string, sortDesc bool) (int, float64, error) {
	encoded, err := s.rdb.HGet(ctx, MemberKey(rankID, typeID, itemID), "member").Result()
	if err == goredis.Nil { return -1, 0, nil }
	if err != nil { return 0, 0, err }
	var rank int64
	if sortDesc { rank, err = s.rdb.ZRevRank(ctx, ZSetKey(rankID, typeID), encoded).Result() } else { rank, err = s.rdb.ZRank(ctx, ZSetKey(rankID, typeID), encoded).Result() }
	if err == goredis.Nil { return -1, 0, nil }
	if err != nil { return 0, 0, err }
	score, err := s.GetScore(ctx, rankID, typeID, itemID)
	if err != nil { return 0, 0, err }
	return int(rank)+1, float64(score), nil
}

func (s *Store) Around(ctx context.Context, rankID int64, typeID, itemID string, before, after int, sortDesc bool) ([]RankEntry, error) {
	rank, _, err := s.MemberRank(ctx, rankID, typeID, itemID, sortDesc)
	if err != nil { return nil, err }
	if rank < 0 { return []RankEntry{}, nil }
	start := rank - 1 - before
	if start < 0 { start = 0 }
	return s.Top(ctx, rankID, typeID, start, before+after+1, sortDesc)
}

func (s *Store) Card(ctx context.Context, rankID int64, typeID string) (int64, error) {
	return s.rdb.ZCard(ctx, ZSetKey(rankID, typeID)).Result()
}

func (s *Store) HasBoard(ctx context.Context, rankID int64, typeID string) (bool, error) {
	n, err := s.rdb.Exists(ctx, ZSetKey(rankID, typeID)).Result()
	return n > 0, err
}

func (s *Store) SetConfigCache(ctx context.Context, rankID int64, payload string, ttl time.Duration) error { return s.rdb.Set(ctx, ConfigKey(rankID), payload, ttl).Err() }
func (s *Store) GetConfigCache(ctx context.Context, rankID int64) (string, bool, error) {
	v, err := s.rdb.Get(ctx, ConfigKey(rankID)).Result()
	if err == goredis.Nil { return "", false, nil }
	if err != nil { return "", false, err }
	return v, true, nil
}
func (s *Store) DelConfigCache(ctx context.Context, rankID int64) error { return s.rdb.Del(ctx, ConfigKey(rankID)).Err() }

// MigrateLegacyListQueue moves any pre-stream jobs into the stream without
// dropping them. It can safely run at every startup.
func (s *Store) MigrateLegacyListQueue(ctx context.Context) error {
	for {
		payload, err := s.rdb.RPop(ctx, legacyPersistQueue).Result()
		if err == goredis.Nil { return nil }
		if err != nil { return err }
		if err := s.EnqueuePersist(ctx, payload); err != nil {
			_ = s.rdb.RPush(ctx, legacyPersistQueue, payload).Err()
			return err
		}
	}
}

func (s *Store) EnsurePersistGroup(ctx context.Context) error {
	err := s.rdb.XGroupCreateMkStream(ctx, PersistStreamKey, PersistGroupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") { return err }
	return nil
}

func (s *Store) EnqueuePersist(ctx context.Context, payload string) error {
	return s.rdb.XAdd(ctx, &goredis.XAddArgs{Stream: PersistStreamKey, Values: map[string]interface{}{"payload": payload}}).Err()
}

type PersistMessage struct { ID, Payload string }

func (s *Store) ReadPersist(ctx context.Context, consumer string, pending bool, block time.Duration) ([]PersistMessage, error) {
	start := ">"
	if pending { start = "0" }
	streams, err := s.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{Group: PersistGroupName, Consumer: consumer, Streams: []string{PersistStreamKey, start}, Count: 1, Block: block}).Result()
	if err == goredis.Nil { return nil, nil }
	if err != nil { return nil, err }
	out := make([]PersistMessage, 0, 1)
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			payload, ok := msg.Values["payload"]
			if !ok { return nil, errors.New("persist stream message missing payload") }
			out = append(out, PersistMessage{ID: msg.ID, Payload: fmt.Sprint(payload)})
		}
	}
	return out, nil
}

func (s *Store) AckPersist(ctx context.Context, id string) error { return s.rdb.XAck(ctx, PersistStreamKey, PersistGroupName, id).Err() }
