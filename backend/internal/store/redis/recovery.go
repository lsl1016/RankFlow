package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

var restoreMemberIfNewerScript = goredis.NewScript(`
local zkey = KEYS[1]
local mkey = KEYS[2]
local member = ARGV[1]
local businessScore = tonumber(ARGV[2])
local revision = tonumber(ARGV[3])
local finalScore = tonumber(ARGV[4])

local currentRevision = tonumber(redis.call('HGET', mkey, 'revision') or '-1')
if currentRevision > revision then
  return 0
end
local oldMember = redis.call('HGET', mkey, 'member')
if oldMember and oldMember ~= member then
  redis.call('ZREM', zkey, oldMember)
end
redis.call('HSET', mkey,
  'score', businessScore,
  'revision', revision,
  'member', member,
  'final', finalScore)
redis.call('ZADD', zkey, businessScore, member)
return 1
`)

// RestoreMemberIfNewer rebuilds Redis from a persisted MySQL snapshot without
// allowing a stale snapshot to overwrite a concurrent newer Redis write.
func (s *Store) RestoreMemberIfNewer(ctx context.Context, rankID int64, typeID, itemID, encodedMember string, businessScore, revision int64, finalScore float64) error {
	return restoreMemberIfNewerScript.Run(ctx, s.rdb,
		[]string{ZSetKey(rankID, typeID), MemberKey(rankID, typeID, itemID)},
		encodedMember, businessScore, revision, finalScore,
	).Err()
}
