package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	WriteModeAdd = "add"
	WriteModeSet = "set"
)

var ErrLegacyIdempotencyRecord = errors.New("legacy idempotency record")

type IdempotencyRecord struct {
	Fingerprint string
	TypeID      string
	ItemID      string
	Score       int64
	Revision    int64
	Final       float64
}

type AtomicWriteRequest struct {
	Mode           string
	RankID         int64
	TypeID         string
	ItemID         string
	EncodedMember  string
	Value          int64
	SubScore       int64
	EventTime      int64
	SubDecimal     float64
	MaxSize        int
	SortDesc       bool
	RequestID      string
	Fingerprint    string
	IdempotencyTTL time.Duration
	TraceID        string
}

type AtomicWriteResult struct {
	Applied   bool
	Duplicate bool
	Conflict  bool
	TypeID    string
	ItemID    string
	Score     int64
	Revision  int64
	Final     float64
}

func (s *Store) GetIdempotencyRecord(ctx context.Context, rankID int64, requestID string) (*IdempotencyRecord, bool, error) {
	if requestID == "" {
		return nil, false, nil
	}
	key := IdemKey(rankID, requestID)
	keyType, err := s.rdb.Type(ctx, key).Result()
	if err != nil {
		return nil, false, err
	}
	switch keyType {
	case "none":
		return nil, false, nil
	case "hash":
		values, err := s.rdb.HGetAll(ctx, key).Result()
		if err != nil {
			return nil, false, err
		}
		if values["fingerprint"] == "" {
			return nil, true, fmt.Errorf("idempotency record %q is missing fingerprint", key)
		}
		score, err := strconv.ParseInt(values["score"], 10, 64)
		if err != nil {
			return nil, true, fmt.Errorf("parse idempotency score: %w", err)
		}
		revision, err := strconv.ParseInt(values["revision"], 10, 64)
		if err != nil {
			return nil, true, fmt.Errorf("parse idempotency revision: %w", err)
		}
		final, err := strconv.ParseFloat(values["final"], 64)
		if err != nil {
			return nil, true, fmt.Errorf("parse idempotency final score: %w", err)
		}
		return &IdempotencyRecord{
			Fingerprint: values["fingerprint"],
			TypeID:      values["typeId"],
			ItemID:      values["itemId"],
			Score:       score,
			Revision:    revision,
			Final:       final,
		}, true, nil
	default:
		// Older RankFlow versions stored idempotency as a SET string. Treat it
		// as an opaque legacy claim so an upgrade never replays an increment.
		return nil, true, ErrLegacyIdempotencyRecord
	}
}

var atomicWriteScript = goredis.NewScript(`
local zkey = KEYS[1]
local mkey = KEYS[2]
local idemkey = KEYS[3]
local streamkey = KEYS[4]

local mode = ARGV[1]
local encodedMember = ARGV[2]
local value = tonumber(ARGV[3])
local maxsize = tonumber(ARGV[4])
local sortDesc = ARGV[5]
local subdecimal = tonumber(ARGV[6])
local requestID = ARGV[7]
local fingerprint = ARGV[8]
local ttlSeconds = tonumber(ARGV[9])
local rankID = ARGV[10]
local typeID = ARGV[11]
local itemID = ARGV[12]
local subScore = ARGV[13]
local eventTime = ARGV[14]
local traceID = ARGV[15]
local maxExactScore = 9007199254740991

local function keyType(key)
  return redis.call('TYPE', key).ok
end

local ztype = keyType(zkey)
if ztype ~= 'none' and ztype ~= 'zset' then
  return redis.error_reply('rank zset key has incompatible type: ' .. ztype)
end
local mtype = keyType(mkey)
if mtype ~= 'none' and mtype ~= 'hash' then
  return redis.error_reply('rank member key has incompatible type: ' .. mtype)
end
local stype = keyType(streamkey)
if stype ~= 'none' and stype ~= 'stream' then
  return redis.error_reply('persist stream key has incompatible type: ' .. stype)
end

if requestID ~= '' then
  local itype = keyType(idemkey)
  if itype == 'string' then
    return {'legacy'}
  end
  if itype ~= 'none' and itype ~= 'hash' then
    return redis.error_reply('idempotency key has incompatible type: ' .. itype)
  end
  local existingFingerprint = redis.call('HGET', idemkey, 'fingerprint')
  if existingFingerprint then
    local status = 'duplicate'
    if existingFingerprint ~= fingerprint then
      status = 'conflict'
    end
    return {
      status,
      redis.call('HGET', idemkey, 'score') or '0',
      redis.call('HGET', idemkey, 'revision') or '0',
      redis.call('HGET', idemkey, 'final') or '0',
      redis.call('HGET', idemkey, 'typeId') or '',
      redis.call('HGET', idemkey, 'itemId') or ''
    }
  end
end

local cur = tonumber(redis.call('HGET', mkey, 'score') or '0')
local newScore
if mode == 'add' then
  newScore = cur + value
elseif mode == 'set' then
  newScore = value
else
  return redis.error_reply('unsupported score write mode: ' .. mode)
end
if newScore > maxExactScore or newScore < -maxExactScore then
  return {'range'}
end

local oldMember = redis.call('HGET', mkey, 'member')
local revision = tonumber(redis.call('HGET', mkey, 'revision') or '0') + 1
local finalScore = newScore + subdecimal
local scoreText = string.format('%.0f', newScore)
local revisionText = string.format('%.0f', revision)
local finalText = tostring(finalScore)

if oldMember then
  redis.call('ZREM', zkey, oldMember)
end
redis.call('HSET', mkey,
  'score', scoreText,
  'revision', revisionText,
  'member', encodedMember,
  'final', finalText)
redis.call('ZADD', zkey, newScore, encodedMember)

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

redis.call('XADD', streamkey, '*',
  'format', 'v2',
  'rankId', rankID,
  'typeId', typeID,
  'itemId', itemID,
  'traceId', traceID,
  'score', scoreText,
  'subScore', subScore,
  'final', finalText,
  'revision', revisionText,
  'eventTime', eventTime)

if requestID ~= '' then
  redis.call('HSET', idemkey,
    'fingerprint', fingerprint,
    'typeId', typeID,
    'itemId', itemID,
    'score', scoreText,
    'revision', revisionText,
    'final', finalText)
  redis.call('EXPIRE', idemkey, ttlSeconds)
end

return {'applied', scoreText, revisionText, finalText, typeID, itemID}
`)

func (s *Store) WriteScoreAtomic(ctx context.Context, in AtomicWriteRequest) (*AtomicWriteResult, error) {
	descFlag := "0"
	if in.SortDesc {
		descFlag = "1"
	}
	ttl := int64(in.IdempotencyTTL / time.Second)
	if ttl <= 0 {
		ttl = int64((24 * time.Hour) / time.Second)
	}
	res, err := atomicWriteScript.Run(ctx, s.rdb,
		[]string{
			ZSetKey(in.RankID, in.TypeID),
			MemberKey(in.RankID, in.TypeID, in.ItemID),
			IdemKey(in.RankID, in.RequestID),
			PersistStreamKey,
		},
		in.Mode,
		in.EncodedMember,
		in.Value,
		in.MaxSize,
		descFlag,
		in.SubDecimal,
		in.RequestID,
		in.Fingerprint,
		ttl,
		in.RankID,
		in.TypeID,
		in.ItemID,
		in.SubScore,
		in.EventTime,
		in.TraceID,
	).Result()
	if err != nil {
		return nil, err
	}
	vals, ok := res.([]interface{})
	if !ok || len(vals) == 0 {
		return nil, fmt.Errorf("unexpected atomic score result: %#v", res)
	}
	status := fmt.Sprint(vals[0])
	if status == "legacy" {
		return nil, ErrLegacyIdempotencyRecord
	}
	if status == "range" {
		return &AtomicWriteResult{}, fmt.Errorf("score exceeds exact Redis range")
	}
	if len(vals) != 6 {
		return nil, fmt.Errorf("unexpected atomic score result: %#v", res)
	}
	score, err := strconv.ParseInt(fmt.Sprint(vals[1]), 10, 64)
	if err != nil {
		return nil, err
	}
	revision, err := strconv.ParseInt(fmt.Sprint(vals[2]), 10, 64)
	if err != nil {
		return nil, err
	}
	final, err := strconv.ParseFloat(fmt.Sprint(vals[3]), 64)
	if err != nil {
		return nil, err
	}
	return &AtomicWriteResult{
		Applied:   status == "applied",
		Duplicate: status == "duplicate",
		Conflict:  status == "conflict",
		TypeID:    fmt.Sprint(vals[4]),
		ItemID:    fmt.Sprint(vals[5]),
		Score:     score,
		Revision:  revision,
		Final:     final,
	}, nil
}
