package redis

import (
	"context"
	"fmt"

	"github.com/interview-eval/backend/internal/config"
	redispkg "github.com/redis/go-redis/v9"
)

// NewClient dials Redis using cfg and verifies connectivity with PING.
func NewClient(ctx context.Context, cfg *config.Config) (*redispkg.Client, error) {
	rdb := redispkg.NewClient(&redispkg.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}

// EvaluationCacheKey returns the Redis key for cached AI evaluation JSON.
func EvaluationCacheKey(answerID string) string {
	return fmt.Sprintf("evaluation:%s", answerID)
}
