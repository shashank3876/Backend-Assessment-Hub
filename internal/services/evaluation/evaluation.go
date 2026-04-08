package evaluation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/interview-eval/backend/internal/config"
	"github.com/interview-eval/backend/internal/db"
	"github.com/interview-eval/backend/internal/models"
	appredis "github.com/interview-eval/backend/internal/redis"
	redispkg "github.com/redis/go-redis/v9"
	openai "github.com/sashabaranov/go-openai"
)

const maxRetries = 3

type Service struct {
	db       *db.DB
	client   *openai.Client
	rdb      *redispkg.Client
	cacheTTL time.Duration
}

type aiEvalResult struct {
	Score              float64  `json:"score"`
	Feedback           string   `json:"feedback"`
	Strengths          []string `json:"strengths"`
	AreasOfImprovement []string `json:"areas_of_improvement"`
}

func New(database *db.DB, cfg *config.Config, rdb *redispkg.Client) *Service {
	clientConfig := openai.DefaultConfig(cfg.OpenAIAPIKey)
	if cfg.OpenAIBaseURL != "" {
		clientConfig.BaseURL = cfg.OpenAIBaseURL
	}
	client := openai.NewClientWithConfig(clientConfig)
	return &Service{db: database, client: client, rdb: rdb, cacheTTL: cfg.EvaluationCacheTTL}
}

func (s *Service) EvaluateAnswer(ctx context.Context, job models.EvaluationJob) error {
	slog.Info("evaluating answer", "answerId", job.AnswerID, "retry", job.RetryCount)

	if _, err := s.db.GetEvaluationByAnswerID(ctx, job.AnswerID); err == nil {
		if err := s.db.UpdateAnswerStatus(ctx, job.AnswerID, string(models.AnswerCompleted)); err != nil {
			return fmt.Errorf("failed to update answer status: %w", err)
		}
		slog.Info("evaluation already persisted, skipping duplicate work", "answerId", job.AnswerID)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing evaluation: %w", err)
	}

	if err := s.db.UpdateAnswerStatus(ctx, job.AnswerID, string(models.AnswerProcessing)); err != nil {
		return fmt.Errorf("failed to update answer status: %w", err)
	}

	var (
		result *aiEvalResult
		err    error
	)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			slog.Warn("retrying AI evaluation", "answerId", job.AnswerID, "attempt", attempt)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		result, err = s.callAI(ctx, job)
		if err == nil {
			break
		}
		slog.Error("AI evaluation attempt failed", "error", err, "attempt", attempt)
	}

	if err != nil {
		_ = s.db.UpdateAnswerStatus(ctx, job.AnswerID, string(models.AnswerFailed))
		return fmt.Errorf("AI evaluation failed after retries: %w", err)
	}

	_, dbErr := s.db.CreateEvaluation(ctx,
		job.AnswerID,
		result.Score,
		result.Feedback,
		result.Strengths,
		result.AreasOfImprovement,
		job.RetryCount,
	)
	if dbErr != nil {
		_ = s.db.UpdateAnswerStatus(ctx, job.AnswerID, string(models.AnswerFailed))
		return fmt.Errorf("failed to save evaluation: %w", dbErr)
	}

	if err := s.db.UpdateAnswerStatus(ctx, job.AnswerID, string(models.AnswerCompleted)); err != nil {
		return fmt.Errorf("failed to mark answer completed: %w", err)
	}

	slog.Info("answer evaluated successfully", "answerId", job.AnswerID, "score", result.Score)
	return nil
}

func (s *Service) callAI(ctx context.Context, job models.EvaluationJob) (*aiEvalResult, error) {
	cacheKey := appredis.EvaluationCacheKey(job.AnswerID)

	if s.rdb != nil {
		val, err := s.rdb.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var cached aiEvalResult
			if err := json.Unmarshal(val, &cached); err == nil {
				slog.Info("evaluation cache hit", "answerId", job.AnswerID)
				return &cached, nil
			}
			slog.Warn("evaluation cache corrupt, ignoring", "answerId", job.AnswerID, "error", err)
		} else if !errors.Is(err, redispkg.Nil) {
			slog.Warn("evaluation cache get failed", "answerId", job.AnswerID, "error", err)
		}
	}

	systemPrompt := `You are an expert technical interviewer and evaluator. 
Evaluate the candidate's answer to the interview question strictly and fairly.
Return ONLY a valid JSON object with these exact fields:
{
  "score": <number 0-100>,
  "feedback": "<detailed constructive feedback paragraph>",
  "strengths": ["<strength 1>", "<strength 2>", ...],
  "areas_of_improvement": ["<improvement 1>", "<improvement 2>", ...]
}
Do not include any text outside the JSON.`

	userPrompt := fmt.Sprintf("Interview Question: %s\n\nCandidate's Answer: %s", "(unknown)", job.AnswerText)

	if job.QuestionID != "" {
		q, err := s.db.GetQuestionByID(ctx, job.QuestionID)
		if err == nil {
			userPrompt = fmt.Sprintf("Interview Question: %s\n\nCandidate's Answer: %s", q.Text, job.AnswerText)
			if q.ExpectedAnswer != nil && *q.ExpectedAnswer != "" {
				userPrompt += fmt.Sprintf("\n\nExpected Answer Context: %s", *q.ExpectedAnswer)
			}
		}
	}

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-5.2",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		MaxCompletionTokens: 8192,
	})
	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from AI")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1]
		}
		content = strings.Join(lines, "\n")
	}

	var result aiEvalResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w (raw: %s)", err, content)
	}

	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}
	if result.Strengths == nil {
		result.Strengths = []string{}
	}
	if result.AreasOfImprovement == nil {
		result.AreasOfImprovement = []string{}
	}

	if s.rdb != nil && s.cacheTTL > 0 {
		payload, err := json.Marshal(&result)
		if err != nil {
			slog.Warn("marshal evaluation for cache failed", "answerId", job.AnswerID, "error", err)
		} else if err := s.rdb.Set(ctx, cacheKey, payload, s.cacheTTL).Err(); err != nil {
			slog.Warn("evaluation cache set failed", "answerId", job.AnswerID, "error", err)
		}
	}

	return &result, nil
}
