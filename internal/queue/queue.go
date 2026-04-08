package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/interview-eval/backend/internal/config"
	"github.com/interview-eval/backend/internal/models"
	redispkg "github.com/redis/go-redis/v9"
)

// Queue is a Redis list-backed job queue (LPUSH producer, BRPOP consumers).
type Queue struct {
	rdb *redispkg.Client
	key string
}

// New constructs a queue backed by Redis lists.
func New(rdb *redispkg.Client, cfg *config.Config) *Queue {
	return &Queue{rdb: rdb, key: cfg.RedisQueueKey}
}

// Publish pushes a job onto the queue (left push).
func (q *Queue) Publish(ctx context.Context, job models.EvaluationJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal evaluation job: %w", err)
	}
	if err := q.rdb.LPush(ctx, q.key, payload).Err(); err != nil {
		return fmt.Errorf("redis LPUSH: %w", err)
	}
	slog.Info("job published to redis queue", "answerId", job.AnswerID, "queueKey", q.key)
	return nil
}

// Pop blocks up to block waiting for a job (right pop). Returns redis.Nil when the block times out with no element.
func (q *Queue) Pop(ctx context.Context, block time.Duration) (models.EvaluationJob, error) {
	res, err := q.rdb.BRPop(ctx, block, q.key).Result()
	if err != nil {
		var zero models.EvaluationJob
		return zero, err
	}
	if len(res) < 2 {
		return models.EvaluationJob{}, fmt.Errorf("unexpected BRPOP result length: %d", len(res))
	}
	var job models.EvaluationJob
	if err := json.Unmarshal([]byte(res[1]), &job); err != nil {
		return models.EvaluationJob{}, fmt.Errorf("unmarshal evaluation job: %w", err)
	}
	return job, nil
}
