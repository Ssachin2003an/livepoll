
package live

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "strconv"
    "strings"
    "time"

	"github.com/redis/go-redis/v9"
	"livepoll/backend/internal/model"
	"livepoll/backend/internal/store"
)

var ErrRebuilding = errors.New("redis counters are being reconciled")

type Live struct {
	R *redis.Client
}

type CounterSnapshot struct {
	Total  int64
	Counts map[string]int64
}

func Connect(raw string) (*Live, error) {
	options, err := redis.ParseURL(raw)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(options)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err = client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Live{R: client}, nil
}

func countKey(pollID string) string {
	return fmt.Sprintf("poll:%s:counts", pollID)
}

func seenKey(pollID string) string {
	return fmt.Sprintf("poll:%s:processed-votes", pollID)
}

func lockKey(pollID string) string {
	return fmt.Sprintf("poll:%s:rebuild-lock", pollID)
}

// A ballot ID is recorded once. Each distinct option is incremented once,
// while __total is incremented only for the first option of that ballot.
var voteScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[3]) == 1 then
  return redis.error_reply('REBUILDING')
end

local voteID = ARGV[1]
local optionCount = tonumber(ARGV[2])
local ballotMarker = voteID .. ':__ballot'
local ballotAdded = redis.call('SADD', KEYS[2], ballotMarker)

if ballotAdded == 1 then
  redis.call('HINCRBY', KEYS[1], '__total', 1)
end

local pos = 3
for i = 1, optionCount do
  local optionID = ARGV[pos]
  pos = pos + 1

  local added = redis.call('SADD', KEYS[2], voteID .. ':' .. optionID)
  if added == 1 then
    redis.call('HINCRBY', KEYS[1], optionID, 1)
  end
end

local total = redis.call('HGET', KEYS[1], '__total') or '0'
return {ballotAdded, total}
`)

var rateLimitScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return count
`)

func (l *Live) Allow(
	ctx context.Context,
	key string,
	limit int64,
	window time.Duration,
) (bool, error) {
	count, err := rateLimitScript.Run(
		ctx,
		l.R,
		[]string{"rate:" + key},
		int64(window.Seconds()),
	).Int64()

	return count <= limit, err
}

// Vote remains compatible with the existing single-option API.
func (l *Live) Vote(
    ctx context.Context,
    pollID, optionID, voteID string,
) (bool, int64, int64, error) {
    err := l.VoteOptions(ctx, pollID, voteID, []string{optionID})
    if err != nil {
        return false, 0, 0, err
    }

    snapshot, err := l.Snapshot(ctx, pollID)
    if err != nil {
        return false, 0, 0, err
    }

    return true, snapshot.Counts[optionID], snapshot.Total, nil
}

// VoteOptions atomically records one ballot and its selected options.
func (l *Live) VoteOptions(
    ctx context.Context,
    pollID, voteID string,
    optionIDs []string,
) error {
    if pollID == "" || voteID == "" || len(optionIDs) == 0 {
        return errors.New("poll ID, vote ID and options are required")
    }

    args := make([]any, 0, len(optionIDs)+2)
    args = append(args, voteID, len(optionIDs))

    seen := make(map[string]bool, len(optionIDs))
    for _, optionID := range optionIDs {
        if optionID == "" {
            return errors.New("option ID is required")
        }
        if seen[optionID] {
            return errors.New("duplicate option ID")
        }
        seen[optionID] = true
        args = append(args, optionID)
    }

    _, err := voteScript.Run(
        ctx,
        l.R,
        []string{
            countKey(pollID),
            seenKey(pollID),
            lockKey(pollID),
        },
        args...,
    ).Slice()

    if err != nil && strings.Contains(err.Error(), "REBUILDING") {
        return ErrRebuilding
    }

    return err
}

func redisInt64(value any) (int64, error) {
	switch number := value.(type) {
	case int64:
		return number, nil
	case string:
		return strconv.ParseInt(number, 10, 64)
	case []byte:
		return strconv.ParseInt(string(number), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}

func (l *Live) Snapshot(
	ctx context.Context,
	pollID string,
) (CounterSnapshot, error) {
	fields, err := l.R.HGetAll(ctx, countKey(pollID)).Result()
	if err != nil {
		return CounterSnapshot{}, err
	}

	if len(fields) == 0 {
		return CounterSnapshot{}, redis.Nil
	}

	total, err := strconv.ParseInt(fields["__total"], 10, 64)
	if err != nil {
		return CounterSnapshot{}, err
	}

	counts := make(map[string]int64, len(fields)-1)
	for optionID, raw := range fields {
		if optionID == "__total" {
			continue
		}

		count, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			return CounterSnapshot{}, parseErr
		}
		counts[optionID] = count
	}

	return CounterSnapshot{Total: total, Counts: counts}, nil
}


func (l *Live) Seed(
	ctx context.Context,
	results model.Results,
	votes []store.VoteSnapshot,
) error {
	pollID := results.Poll.ID
	lock := lockKey(pollID)

	acquired, err := l.R.SetNX(ctx, lock, "1", 10*time.Second).Result()
	if err != nil {
		return err
	}
	if !acquired {
		return ErrRebuilding
	}
	defer l.R.Del(context.Background(), lock)

	args := make([]any, 0, 3+len(results.Counts)*2+len(votes))
	args = append(args, results.TotalVotes, len(results.Counts))

	for optionID, count := range results.Counts {
		args = append(args, optionID, count)
	}

	args = append(args, len(votes))
	for _, vote := range votes {
		args = append(args, vote.ID, len(vote.OptionIDs))
		for _, optionID := range vote.OptionIDs {
			args = append(args, optionID)
		}
	}

	rebuild := redis.NewScript(`
local current = tonumber(redis.call('HGET', KEYS[1], '__total') or '0')
local incoming = tonumber(ARGV[1])
if current > incoming then return 0 end

redis.call('DEL', KEYS[1], KEYS[2])
redis.call('HSET', KEYS[1], '__total', incoming)

local n = tonumber(ARGV[2])
local pos = 3
for i = 1, n do
  redis.call('HSET', KEYS[1], ARGV[pos], ARGV[pos + 1])
  pos = pos + 2
end

local voteCount = tonumber(ARGV[pos])
pos = pos + 1

for i = 1, voteCount do
	local voteID = ARGV[pos]
	local optionCount = tonumber(ARGV[pos + 1])
	pos = pos + 2

	redis.call('SADD', KEYS[2], voteID .. ':__ballot')

	for j = 1, optionCount do
		local optionID = ARGV[pos]
		redis.call('SADD', KEYS[2], voteID .. ':' .. optionID)
		pos = pos + 1
	end
end

return 1
`)

	_, err = rebuild.Run(
		ctx,
		l.R,
		[]string{countKey(pollID), seenKey(pollID)},
		args...,
	).Result()

	return err
}

func (l *Live) DeletePollData(ctx context.Context, pollID string) error {
    return l.R.Del(
        ctx,
        countKey(pollID),
        seenKey(pollID),
        lockKey(pollID),
    ).Err()
}

func (l *Live) Publish(
	ctx context.Context,
	pollID string,
	event any,
) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return l.R.Publish(ctx, "poll:"+pollID+":events", payload).Err()
}

func (l *Live) Subscribe(ctx context.Context, pollID string) *redis.PubSub {
	return l.R.Subscribe(ctx, "poll:"+pollID+":events")
}

func (l *Live) Close() error {
	return l.R.Close()
}
