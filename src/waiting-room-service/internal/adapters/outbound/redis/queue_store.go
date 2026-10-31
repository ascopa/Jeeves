package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const positionTTL = 24 * time.Hour

func tailKey(eventID uuid.UUID) string { return fmt.Sprintf("wr:{%s}:tail", eventID) }
func headKey(eventID uuid.UUID) string { return fmt.Sprintf("wr:{%s}:head", eventID) }

func tickKey(eventID uuid.UUID, tick int64) string {
	return fmt.Sprintf("wr:{%s}:tick:%d", eventID, tick)
}

func rateKey(eventID uuid.UUID) string { return fmt.Sprintf("wr:{%s}:rate", eventID) }

func activeKey(eventID uuid.UUID) string { return fmt.Sprintf("wr:{%s}:active", eventID) }
func positionKey(eventID, userID uuid.UUID) string {
	return fmt.Sprintf("wr:{%s}:pos:%s", eventID, userID)
}
func admittedKey(eventID, userID uuid.UUID) string {
	return fmt.Sprintf("wr:{%s}:admitted:%s", eventID, userID)
}

var joinScript = goredis.NewScript(`
local pos = redis.call('GET', KEYS[1])
if pos then return tonumber(pos) end
pos = redis.call('INCR', KEYS[2])
redis.call('SET', KEYS[1], pos, 'EX', ARGV[1])
return pos
`)

var admitScript = goredis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[3], '-inf', ARGV[3])
local head = tonumber(redis.call('GET', KEYS[1]) or '0')
local tail = tonumber(redis.call('GET', KEYS[2]) or '0')
local n = math.max(0, math.min(tonumber(ARGV[1]) - redis.call('ZCARD', KEYS[3]), tonumber(ARGV[2]), tail - head))
if tail > head then
  local rate = tonumber(redis.call('GET', KEYS[4]) or ARGV[2])
  redis.call('SET', KEYS[4], tostring(math.max(0.9 * rate + 0.1 * n, 0.1)))
end
if n == 0 then return 0 end
local expires = tonumber(ARGV[3]) + tonumber(ARGV[4])
for pos = head + 1, head + n do
  redis.call('ZADD', KEYS[3], expires, pos)
end
redis.call('SET', KEYS[1], head + n)
return n
`)

var admissionScript = goredis.NewScript(`
local expires = redis.call('GET', KEYS[1])
if expires then return tonumber(expires) end
expires = redis.call('ZSCORE', KEYS[2], ARGV[1])
if not expires then return 0 end
redis.call('SET', KEYS[1], expires, 'EX', ARGV[2])
return tonumber(expires)
`)

var releaseScript = goredis.NewScript(`
local pos = redis.call('GET', KEYS[1])
if pos then redis.call('ZREM', KEYS[2], pos) end
return 0
`)

type QueueStore struct {
	client *goredis.Client
}

func NewQueueStore(client *goredis.Client) *QueueStore {
	return &QueueStore{client: client}
}

func (s *QueueStore) Join(ctx context.Context, eventID, userID uuid.UUID) (int64, error) {
	return joinScript.Run(ctx, s.client,
		[]string{positionKey(eventID, userID), tailKey(eventID)},
		int64(positionTTL.Seconds()),
	).Int64()
}

func (s *QueueStore) Position(ctx context.Context, eventID, userID uuid.UUID) (int64, error) {
	return getInt(ctx, s.client, positionKey(eventID, userID))
}

func (s *QueueStore) Leave(ctx context.Context, eventID, userID uuid.UUID) error {
	return s.client.Del(ctx, positionKey(eventID, userID), admittedKey(eventID, userID)).Err()
}

func (s *QueueStore) Progress(ctx context.Context, eventID uuid.UUID) (int64, float64, error) {
	values, err := s.client.MGet(ctx, headKey(eventID), rateKey(eventID)).Result()
	if err != nil {
		return 0, 0, err
	}
	var head int64
	var rate float64
	if v, ok := values[0].(string); ok {
		if head, err = strconv.ParseInt(v, 10, 64); err != nil {
			return 0, 0, err
		}
	}
	if v, ok := values[1].(string); ok {
		if rate, err = strconv.ParseFloat(v, 64); err != nil {
			return 0, 0, err
		}
	}
	return head, rate, nil
}

func (s *QueueStore) ClaimTick(ctx context.Context, eventID uuid.UUID, now time.Time, interval time.Duration) (bool, error) {
	tick := now.UnixMilli() / interval.Milliseconds()

	return s.client.SetNX(ctx, tickKey(eventID, tick), 1, 2*interval).Result()
}

func (s *QueueStore) Admit(ctx context.Context, eventID uuid.UUID, target, ceiling int, now time.Time, window time.Duration) (int, error) {
	return admitScript.Run(ctx, s.client,
		[]string{headKey(eventID), tailKey(eventID), activeKey(eventID), rateKey(eventID)},
		target, ceiling, now.UnixMilli(), window.Milliseconds(),
	).Int()
}

func (s *QueueStore) AdmissionExpiry(ctx context.Context, eventID, userID uuid.UUID, position int64) (time.Time, error) {
	millis, err := admissionScript.Run(ctx, s.client,
		[]string{admittedKey(eventID, userID), activeKey(eventID)},
		position, int64(positionTTL.Seconds()),
	).Int64()
	if err != nil || millis == 0 {
		return time.Time{}, err
	}
	return time.UnixMilli(millis), nil
}

func (s *QueueStore) Release(ctx context.Context, eventID, userID uuid.UUID) error {
	return releaseScript.Run(ctx, s.client, []string{positionKey(eventID, userID), activeKey(eventID)}).Err()
}

func getInt(ctx context.Context, client *goredis.Client, key string) (int64, error) {
	value, err := client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(value, 10, 64)
}
