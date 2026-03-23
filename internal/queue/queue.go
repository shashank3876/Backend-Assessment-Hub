package queue

import (
	"context"
	"log/slog"
	"sync"

	"github.com/interview-eval/backend/internal/models"
)

// Queue is an in-process channel-based message queue.
// It mirrors the publish/consume pattern of RabbitMQ and can be
// replaced with an actual RabbitMQ/NATS client without changing the callers.
type Queue struct {
	ch      chan models.EvaluationJob
	mu      sync.Mutex
	closed  bool
}

func New(bufferSize int) *Queue {
	return &Queue{
		ch: make(chan models.EvaluationJob, bufferSize),
	}
}

// Publish enqueues an evaluation job.
func (q *Queue) Publish(job models.EvaluationJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil
	}
	select {
	case q.ch <- job:
		slog.Info("job published to queue", "answerId", job.AnswerID)
		return nil
	default:
		slog.Warn("queue full, dropping job", "answerId", job.AnswerID)
		return nil
	}
}

// Consume returns a read-only channel of evaluation jobs.
func (q *Queue) Consume() <-chan models.EvaluationJob {
	return q.ch
}

// Close shuts down the queue.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}

// Run starts a pool of workers that consume from the queue.
func (q *Queue) Run(ctx context.Context, concurrency int, handler func(context.Context, models.EvaluationJob) error) {
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			slog.Info("worker started", "workerID", workerID)
			for {
				select {
				case <-ctx.Done():
					slog.Info("worker shutting down", "workerID", workerID)
					return
				case job, ok := <-q.ch:
					if !ok {
						return
					}
					if err := handler(ctx, job); err != nil {
						slog.Error("worker failed to process job", "workerID", workerID, "answerId", job.AnswerID, "error", err)
					}
				}
			}
		}(i)
	}
	<-ctx.Done()
	wg.Wait()
}
