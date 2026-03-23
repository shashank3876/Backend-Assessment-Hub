package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/interview-eval/backend/internal/config"
	"github.com/interview-eval/backend/internal/db"
	"github.com/interview-eval/backend/internal/models"
	openai "github.com/sashabaranov/go-openai"
)

const maxRetries = 3

type Service struct {
	db     *db.DB
	client *openai.Client
}

type aiEvalResult struct {
	Score               float64  `json:"score"`
	Feedback            string   `json:"feedback"`
	Strengths           []string `json:"strengths"`
	AreasOfImprovement  []string `json:"areas_of_improvement"`
}

func New(database *db.DB, cfg *config.Config) *Service {
	clientConfig := openai.DefaultConfig(cfg.OpenAIAPIKey)
	if cfg.OpenAIBaseURL != "" {
		clientConfig.BaseURL = cfg.OpenAIBaseURL
	}
	client := openai.NewClientWithConfig(clientConfig)
	return &Service{db: database, client: client}
}

func (s *Service) EvaluateAnswer(ctx context.Context, job models.EvaluationJob) error {
	slog.Info("evaluating answer", "answerId", job.AnswerID, "retry", job.RetryCount)

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

	userPrompt := fmt.Sprintf(
		"Interview Question: %s\n\nCandidate's Answer: %s",
		job.AnswerText,
		job.AnswerText,
	)

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
	// Strip markdown code fences if present
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

	return &result, nil
}
