package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/interview-eval/backend/internal/models"
	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

func New(databaseURL string) (*DB, error) {
	conn, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) Migrate(ctx context.Context) error {
	_, err := d.conn.ExecContext(ctx, `
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";

		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'candidate' CHECK (role IN ('candidate', 'recruiter', 'admin')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS interviews (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'closed')),
			created_by UUID NOT NULL REFERENCES users(id),
			duration_minutes INTEGER NOT NULL DEFAULT 60,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS questions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
			text TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'text' CHECK (type IN ('text', 'coding', 'behavioral')),
			"order" INTEGER NOT NULL DEFAULT 0,
			max_score REAL NOT NULL DEFAULT 100,
			expected_answer TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS answers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
			interview_id UUID NOT NULL REFERENCES interviews(id) ON DELETE CASCADE,
			candidate_id UUID NOT NULL REFERENCES users(id),
			answer_text TEXT NOT NULL,
			session_id TEXT,
			status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'processing', 'completed', 'failed')),
			submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS evaluations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			answer_id UUID NOT NULL UNIQUE REFERENCES answers(id) ON DELETE CASCADE,
			score REAL NOT NULL,
			feedback TEXT NOT NULL,
			strengths JSONB NOT NULL DEFAULT '[]',
			areas_of_improvement JSONB NOT NULL DEFAULT '[]',
			retry_count INTEGER NOT NULL DEFAULT 0,
			evaluated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE INDEX IF NOT EXISTS idx_answers_candidate_id ON answers(candidate_id);
		CREATE INDEX IF NOT EXISTS idx_answers_interview_id ON answers(interview_id);
		CREATE INDEX IF NOT EXISTS idx_answers_status ON answers(status);
		CREATE INDEX IF NOT EXISTS idx_evaluations_answer_id ON evaluations(answer_id);
		CREATE INDEX IF NOT EXISTS idx_questions_interview_id ON questions(interview_id);
	`)
	return err
}

func (d *DB) CreateUser(ctx context.Context, name, email, passwordHash, role string) (*models.User, error) {
	var u models.User
	err := d.conn.QueryRowContext(ctx,
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1, $2, $3, $4)
		 RETURNING id, name, email, password_hash, role, created_at, updated_at`,
		name, email, passwordHash, role,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := d.conn.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, role, created_at, updated_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	err := d.conn.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, role, created_at, updated_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) ListUsers(ctx context.Context, role string) ([]models.User, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if role != "" {
		rows, err = d.conn.QueryContext(ctx,
			`SELECT id, name, email, password_hash, role, created_at, updated_at FROM users WHERE role = $1 ORDER BY created_at DESC`,
			role)
	} else {
		rows, err = d.conn.QueryContext(ctx,
			`SELECT id, name, email, password_hash, role, created_at, updated_at FROM users ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (d *DB) CreateInterview(ctx context.Context, title string, description *string, status, createdBy string, durationMinutes int) (*models.Interview, error) {
	var iv models.Interview
	err := d.conn.QueryRowContext(ctx,
		`INSERT INTO interviews (title, description, status, created_by, duration_minutes)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, title, description, status, created_by, duration_minutes, created_at, updated_at`,
		title, description, status, createdBy, durationMinutes,
	).Scan(&iv.ID, &iv.Title, &iv.Description, &iv.Status, &iv.CreatedBy, &iv.DurationMinutes, &iv.CreatedAt, &iv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &iv, nil
}

func (d *DB) GetInterviewByID(ctx context.Context, id string) (*models.Interview, error) {
	var iv models.Interview
	err := d.conn.QueryRowContext(ctx,
		`SELECT id, title, description, status, created_by, duration_minutes, created_at, updated_at FROM interviews WHERE id = $1`,
		id,
	).Scan(&iv.ID, &iv.Title, &iv.Description, &iv.Status, &iv.CreatedBy, &iv.DurationMinutes, &iv.CreatedAt, &iv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &iv, nil
}

func (d *DB) ListInterviews(ctx context.Context, status string, page, limit int) ([]models.Interview, int, error) {
	offset := (page - 1) * limit
	var (
		rows     *sql.Rows
		countRow *sql.Row
		err      error
		total    int
	)

	if status != "" {
		countRow = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM interviews WHERE status = $1`, status)
		rows, err = d.conn.QueryContext(ctx,
			`SELECT id, title, description, status, created_by, duration_minutes, created_at, updated_at FROM interviews WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			status, limit, offset)
	} else {
		countRow = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM interviews`)
		rows, err = d.conn.QueryContext(ctx,
			`SELECT id, title, description, status, created_by, duration_minutes, created_at, updated_at FROM interviews ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset)
	}

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	if err := countRow.Scan(&total); err != nil {
		return nil, 0, err
	}

	var interviews []models.Interview
	for rows.Next() {
		var iv models.Interview
		if err := rows.Scan(&iv.ID, &iv.Title, &iv.Description, &iv.Status, &iv.CreatedBy, &iv.DurationMinutes, &iv.CreatedAt, &iv.UpdatedAt); err != nil {
			return nil, 0, err
		}
		interviews = append(interviews, iv)
	}
	return interviews, total, nil
}

func (d *DB) UpdateInterview(ctx context.Context, id string, updates map[string]interface{}) (*models.Interview, error) {
	setParts := ""
	args := []interface{}{}
	i := 1
	for k, v := range updates {
		if i > 1 {
			setParts += ", "
		}
		setParts += fmt.Sprintf("%s = $%d", k, i)
		args = append(args, v)
		i++
	}
	args = append(args, id)
	query := fmt.Sprintf(
		`UPDATE interviews SET %s, updated_at = NOW() WHERE id = $%d RETURNING id, title, description, status, created_by, duration_minutes, created_at, updated_at`,
		setParts, i,
	)
	var iv models.Interview
	err := d.conn.QueryRowContext(ctx, query, args...).Scan(
		&iv.ID, &iv.Title, &iv.Description, &iv.Status, &iv.CreatedBy, &iv.DurationMinutes, &iv.CreatedAt, &iv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &iv, nil
}

func (d *DB) CreateQuestion(ctx context.Context, interviewID, text, qtype string, order int, maxScore float64, expectedAnswer *string) (*models.Question, error) {
	var q models.Question
	err := d.conn.QueryRowContext(ctx,
		`INSERT INTO questions (interview_id, text, type, "order", max_score, expected_answer)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, interview_id, text, type, "order", max_score, expected_answer, created_at`,
		interviewID, text, qtype, order, maxScore, expectedAnswer,
	).Scan(&q.ID, &q.InterviewID, &q.Text, &q.Type, &q.Order, &q.MaxScore, &q.ExpectedAnswer, &q.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (d *DB) GetQuestionsByInterviewID(ctx context.Context, interviewID string) ([]models.Question, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, interview_id, text, type, "order", max_score, expected_answer, created_at FROM questions WHERE interview_id = $1 ORDER BY "order" ASC`,
		interviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		var q models.Question
		if err := rows.Scan(&q.ID, &q.InterviewID, &q.Text, &q.Type, &q.Order, &q.MaxScore, &q.ExpectedAnswer, &q.CreatedAt); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (d *DB) GetQuestionByID(ctx context.Context, id string) (*models.Question, error) {
	var q models.Question
	err := d.conn.QueryRowContext(ctx,
		`SELECT id, interview_id, text, type, "order", max_score, expected_answer, created_at FROM questions WHERE id = $1`,
		id,
	).Scan(&q.ID, &q.InterviewID, &q.Text, &q.Type, &q.Order, &q.MaxScore, &q.ExpectedAnswer, &q.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (d *DB) CreateAnswer(ctx context.Context, questionID, interviewID, candidateID, answerText string, sessionID *string) (*models.Answer, error) {
	var a models.Answer
	err := d.conn.QueryRowContext(ctx,
		`INSERT INTO answers (question_id, interview_id, candidate_id, answer_text, session_id, status)
		 VALUES ($1, $2, $3, $4, $5, 'queued')
		 RETURNING id, question_id, interview_id, candidate_id, answer_text, session_id, status, submitted_at, updated_at`,
		questionID, interviewID, candidateID, answerText, sessionID,
	).Scan(&a.ID, &a.QuestionID, &a.InterviewID, &a.CandidateID, &a.AnswerText, &a.SessionID, &a.Status, &a.SubmittedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) GetAnswerByID(ctx context.Context, id string) (*models.Answer, error) {
	var a models.Answer
	err := d.conn.QueryRowContext(ctx,
		`SELECT id, question_id, interview_id, candidate_id, answer_text, session_id, status, submitted_at, updated_at FROM answers WHERE id = $1`,
		id,
	).Scan(&a.ID, &a.QuestionID, &a.InterviewID, &a.CandidateID, &a.AnswerText, &a.SessionID, &a.Status, &a.SubmittedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (d *DB) UpdateAnswerStatus(ctx context.Context, id, status string) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE answers SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

func (d *DB) GetAnswersByCandidate(ctx context.Context, candidateID, interviewID string) ([]models.Answer, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if interviewID != "" {
		rows, err = d.conn.QueryContext(ctx,
			`SELECT id, question_id, interview_id, candidate_id, answer_text, session_id, status, submitted_at, updated_at FROM answers WHERE candidate_id = $1 AND interview_id = $2 ORDER BY submitted_at DESC`,
			candidateID, interviewID)
	} else {
		rows, err = d.conn.QueryContext(ctx,
			`SELECT id, question_id, interview_id, candidate_id, answer_text, session_id, status, submitted_at, updated_at FROM answers WHERE candidate_id = $1 ORDER BY submitted_at DESC`,
			candidateID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []models.Answer
	for rows.Next() {
		var a models.Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.InterviewID, &a.CandidateID, &a.AnswerText, &a.SessionID, &a.Status, &a.SubmittedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, nil
}

func (d *DB) CreateEvaluation(ctx context.Context, answerID string, score float64, feedback string, strengths, improvements []string, retryCount int) (*models.Evaluation, error) {
	strengthsJSON, _ := json.Marshal(strengths)
	improvementsJSON, _ := json.Marshal(improvements)

	var e models.Evaluation
	var strengthsRaw, improvementsRaw []byte

	err := d.conn.QueryRowContext(ctx,
		`INSERT INTO evaluations (answer_id, score, feedback, strengths, areas_of_improvement, retry_count)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, answer_id, score, feedback, strengths, areas_of_improvement, retry_count, evaluated_at`,
		answerID, score, feedback, strengthsJSON, improvementsJSON, retryCount,
	).Scan(&e.ID, &e.AnswerID, &e.Score, &e.Feedback, &strengthsRaw, &improvementsRaw, &e.RetryCount, &e.EvaluatedAt)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(strengthsRaw, &e.Strengths)
	_ = json.Unmarshal(improvementsRaw, &e.AreasOfImprovement)

	return &e, nil
}

func (d *DB) GetEvaluationByAnswerID(ctx context.Context, answerID string) (*models.Evaluation, error) {
	var e models.Evaluation
	var strengthsRaw, improvementsRaw []byte

	err := d.conn.QueryRowContext(ctx,
		`SELECT id, answer_id, score, feedback, strengths, areas_of_improvement, retry_count, evaluated_at FROM evaluations WHERE answer_id = $1`,
		answerID,
	).Scan(&e.ID, &e.AnswerID, &e.Score, &e.Feedback, &strengthsRaw, &improvementsRaw, &e.RetryCount, &e.EvaluatedAt)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(strengthsRaw, &e.Strengths)
	_ = json.Unmarshal(improvementsRaw, &e.AreasOfImprovement)

	return &e, nil
}

func (d *DB) GetLeaderboard(ctx context.Context, interviewID string) ([]map[string]interface{}, error) {
	rows, err := d.conn.QueryContext(ctx, `
		SELECT
			u.id, u.name, u.email, u.role,
			COALESCE(SUM(ev.score), 0) as total_score,
			COALESCE(AVG(ev.score), 0) as avg_score,
			COUNT(ev.id) as evaluated_count
		FROM users u
		INNER JOIN answers a ON a.candidate_id = u.id AND a.interview_id = $1
		LEFT JOIN evaluations ev ON ev.answer_id = a.id
		WHERE u.role = 'candidate'
		GROUP BY u.id, u.name, u.email, u.role
		HAVING COUNT(a.id) > 0
		ORDER BY total_score DESC
	`, interviewID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var userID, name, email, role string
		var totalScore, avgScore float64
		var count int
		if err := rows.Scan(&userID, &name, &email, &role, &totalScore, &avgScore, &count); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"userId":           userID,
			"name":             name,
			"email":            email,
			"role":             role,
			"totalScore":       totalScore,
			"averageScore":     avgScore,
			"answersEvaluated": count,
		})
	}
	return results, nil
}

func (d *DB) GetAnalyticsOverview(ctx context.Context) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	row := d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM interviews`)
	var totalInterviews int
	_ = row.Scan(&totalInterviews)
	result["totalInterviews"] = totalInterviews

	row = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM interviews WHERE status = 'active'`)
	var activeInterviews int
	_ = row.Scan(&activeInterviews)
	result["activeInterviews"] = activeInterviews

	row = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'candidate'`)
	var totalCandidates int
	_ = row.Scan(&totalCandidates)
	result["totalCandidates"] = totalCandidates

	row = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM answers`)
	var totalAnswers int
	_ = row.Scan(&totalAnswers)
	result["totalAnswersSubmitted"] = totalAnswers

	row = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM evaluations`)
	var totalEvaluated int
	_ = row.Scan(&totalEvaluated)
	result["totalAnswersEvaluated"] = totalEvaluated

	row = d.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM answers WHERE status IN ('queued', 'processing')`)
	var pendingEvals int
	_ = row.Scan(&pendingEvals)
	result["pendingEvaluations"] = pendingEvals

	row = d.conn.QueryRowContext(ctx, `SELECT COALESCE(AVG(score), 0) FROM evaluations`)
	var avgScore float64
	_ = row.Scan(&avgScore)
	result["averageScore"] = avgScore

	return result, nil
}

func (d *DB) GetPendingAnswers(ctx context.Context, limit int) ([]models.Answer, error) {
	rows, err := d.conn.QueryContext(ctx,
		`SELECT id, question_id, interview_id, candidate_id, answer_text, session_id, status, submitted_at, updated_at
		 FROM answers WHERE status = 'queued' ORDER BY submitted_at ASC LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var answers []models.Answer
	for rows.Next() {
		var a models.Answer
		if err := rows.Scan(&a.ID, &a.QuestionID, &a.InterviewID, &a.CandidateID, &a.AnswerText, &a.SessionID, &a.Status, &a.SubmittedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		answers = append(answers, a)
	}
	return answers, nil
}

func (d *DB) IncrementEvaluationRetry(ctx context.Context, answerID string, retryCount int) error {
	_, err := d.conn.ExecContext(ctx,
		`UPDATE evaluations SET retry_count = $1 WHERE answer_id = $2`,
		retryCount, answerID)
	return err
}
