package models

import (
	"time"
)

type UserRole string

const (
	RoleCandidate UserRole = "candidate"
	RoleRecruiter UserRole = "recruiter"
	RoleAdmin     UserRole = "admin"
)

type InterviewStatus string

const (
	StatusDraft  InterviewStatus = "draft"
	StatusActive InterviewStatus = "active"
	StatusClosed InterviewStatus = "closed"
)

type QuestionType string

const (
	QuestionText       QuestionType = "text"
	QuestionCoding     QuestionType = "coding"
	QuestionBehavioral QuestionType = "behavioral"
)

type AnswerStatus string

const (
	AnswerQueued     AnswerStatus = "queued"
	AnswerProcessing AnswerStatus = "processing"
	AnswerCompleted  AnswerStatus = "completed"
	AnswerFailed     AnswerStatus = "failed"
)

type User struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         UserRole  `json:"role" db:"role"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt    time.Time `json:"updatedAt" db:"updated_at"`
}

type Interview struct {
	ID              string          `json:"id" db:"id"`
	Title           string          `json:"title" db:"title"`
	Description     *string         `json:"description" db:"description"`
	Status          InterviewStatus `json:"status" db:"status"`
	CreatedBy       string          `json:"createdBy" db:"created_by"`
	DurationMinutes int             `json:"durationMinutes" db:"duration_minutes"`
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" db:"updated_at"`
}

type InterviewWithQuestions struct {
	Interview
	Questions []Question `json:"questions"`
}

type Question struct {
	ID             string       `json:"id" db:"id"`
	InterviewID    string       `json:"interviewId" db:"interview_id"`
	Text           string       `json:"text" db:"text"`
	Type           QuestionType `json:"type" db:"type"`
	Order          int          `json:"order" db:"order"`
	MaxScore       float64      `json:"maxScore" db:"max_score"`
	ExpectedAnswer *string      `json:"expectedAnswer,omitempty" db:"expected_answer"`
	CreatedAt      time.Time    `json:"createdAt" db:"created_at"`
}

type Answer struct {
	ID          string       `json:"id" db:"id"`
	QuestionID  string       `json:"questionId" db:"question_id"`
	InterviewID string       `json:"interviewId" db:"interview_id"`
	CandidateID string       `json:"candidateId" db:"candidate_id"`
	AnswerText  string       `json:"answerText" db:"answer_text"`
	SessionID   *string      `json:"sessionId,omitempty" db:"session_id"`
	Status      AnswerStatus `json:"status" db:"status"`
	SubmittedAt time.Time    `json:"submittedAt" db:"submitted_at"`
	UpdatedAt   time.Time    `json:"updatedAt" db:"updated_at"`
}

type Evaluation struct {
	ID                  string    `json:"id" db:"id"`
	AnswerID            string    `json:"answerId" db:"answer_id"`
	Score               float64   `json:"score" db:"score"`
	Feedback            string    `json:"feedback" db:"feedback"`
	Strengths           []string  `json:"strengths" db:"strengths"`
	AreasOfImprovement  []string  `json:"areasOfImprovement" db:"areas_of_improvement"`
	RetryCount          int       `json:"retryCount" db:"retry_count"`
	EvaluatedAt         time.Time `json:"evaluatedAt" db:"evaluated_at"`
}

type AnswerWithEvaluation struct {
	Answer     Answer      `json:"answer"`
	Question   Question    `json:"question"`
	Evaluation *Evaluation `json:"evaluation,omitempty"`
}

type CandidateResults struct {
	Candidate        User                   `json:"candidate"`
	TotalScore       float64                `json:"totalScore"`
	AverageScore     float64                `json:"averageScore"`
	AnswersEvaluated int                    `json:"answersEvaluated"`
	Results          []AnswerWithEvaluation `json:"results"`
}

type LeaderboardEntry struct {
	Rank             int     `json:"rank"`
	Candidate        User    `json:"candidate"`
	TotalScore       float64 `json:"totalScore"`
	AverageScore     float64 `json:"averageScore"`
	AnswersEvaluated int     `json:"answersEvaluated"`
}

type AnalyticsOverview struct {
	TotalInterviews      int                `json:"totalInterviews"`
	ActiveInterviews     int                `json:"activeInterviews"`
	TotalCandidates      int                `json:"totalCandidates"`
	TotalAnswersSubmitted int               `json:"totalAnswersSubmitted"`
	TotalAnswersEvaluated int               `json:"totalAnswersEvaluated"`
	PendingEvaluations   int                `json:"pendingEvaluations"`
	AverageScore         float64            `json:"averageScore"`
	TopPerformers        []LeaderboardEntry `json:"topPerformers"`
}

type EvaluationJob struct {
	AnswerID    string
	QuestionID  string
	InterviewID string
	CandidateID string
	AnswerText  string
	RetryCount  int
}
