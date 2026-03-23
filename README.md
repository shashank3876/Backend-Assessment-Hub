
# Project Title

A brief description of what this project does and who it's for

🚀 AI Interview Evaluation Backend

A production-grade backend system for conducting and evaluating technical interviews using AI.
Built with Golang, Gin, PostgreSQL, and OpenAI, featuring async processing, RBAC, observability, and full test coverage.

✨ Features
🔐 JWT-based Authentication (Admin, Recruiter, Candidate)
🧠 AI-powered answer evaluation (async worker + retry)
📊 Leaderboard & analytics
⚡ In-memory queue (RabbitMQ-ready design)
🧵 Worker pool with concurrency control
📈 Prometheus metrics + Grafana dashboards
🛡️ Rate limiting + RBAC enforcement
🐳 Dockerized infrastructure
✅ Comprehensive test suite (20+ test groups)
🛠 Tech Stack
Layer	Technology
Language	Go (Golang)
Framework	Gin
Database	PostgreSQL
Auth	JWT
AI	OpenAI API
Metrics	Prometheus + Grafana
Queue	In-memory (extensible to RabbitMQ)
Container	Docker
🏗 Architecture
Client → API (Gin)
            ↓
        PostgreSQL
            ↓
        Queue (Channel)
            ↓
        Worker Pool
            ↓
        OpenAI API
            ↓
        Evaluation Stored in DB
📁 Project Structure
go-backend/
├── cmd/api/main.go
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   └── router.go
│   ├── config/
│   ├── db/
│   ├── models/
│   ├── queue/
│   ├── services/
│   │   └── evaluation/
│   └── worker/
├── docker-compose.yml
├── Dockerfile
├── prometheus.yml
├── api_test.go
└── README.md
🗄 Database Schema

Core entities:

Users (Admin / Recruiter / Candidate)
Interviews
Questions
Answers
Evaluations

Uses UUIDs for all primary keys.

🔐 Roles & Access
Role	Permissions
Admin	Full access
Recruiter	Create interviews, view analytics
Candidate	Attempt interviews, submit answers
🌍 API Overview
Auth
POST /api/auth/register
POST /api/auth/login
GET /api/auth/me
Interviews
POST /api/interviews (Recruiter)
GET /api/interviews
POST /api/interviews/:id/questions
Candidate Flow
POST /api/interview/start
POST /api/answer/submit
GET /api/answers/:id
Analytics
GET /api/results/:candidateId
GET /api/evaluations/:answerId
GET /api/analytics/overview
System
GET /health
GET /metrics
⚙️ Environment Variables

Create a .env file:

DATABASE_URL=postgres://user:password@localhost:5432/interview_eval?sslmode=disable
GO_PORT=8081
JWT_SECRET=your-secret

AI_INTEGRATIONS_OPENAI_BASE_URL=https://api.openai.com/v1
AI_INTEGRATIONS_OPENAI_API_KEY=your-openai-key
🚀 Getting Started
1. Clone Repo
git clone <your-repo>
cd go-backend
2. Run Locally
export DATABASE_URL=postgres://postgres:postgres@localhost:5432/interview_eval?sslmode=disable
export JWT_SECRET=secret
export AI_INTEGRATIONS_OPENAI_API_KEY=your-key

go run cmd/api/main.go
3. Run with Docker
docker-compose up --build
🧪 Running Tests
go test ./...

With race detector:

go test -race ./...
🧪 Test Coverage
Group	Coverage
Auth	Register, login, token validation
Interviews	CRUD + RBAC
Questions	Validation + role checks
Answers	Async submission + evaluation
Analytics	Leaderboard + results
Security	RBAC, validation
Metrics	Prometheus endpoint
🔄 API Walkthrough (End-to-End)
1. Register recruiter
{
  "name": "Recruiter",
  "email": "rec@test.com",
  "password": "123456",
  "role": "recruiter"
}
2. Create interview
{
  "title": "Backend Interview",
  "description": "Golang + System Design"
}
3. Add question
{
  "text": "Explain goroutines",
  "type": "text"
}
4. Candidate submits answer
{
  "interviewId": "<uuid>",
  "questionId": "<uuid>",
  "answerText": "Goroutines are lightweight threads..."
}
5. Get AI evaluation
{
  "score": 85,
  "feedback": "Good explanation but missing examples"
}
🧠 AI Evaluation Flow
Answer submitted
Job pushed to queue
Worker picks job
Calls OpenAI API
Retries (max 3x)
Stores result
📊 Observability
/metrics → Prometheus
Structured JSON logs
Worker logs with retries
Grafana dashboards supported
⚡ Scalability
Worker pool (configurable concurrency)
Queue abstraction (RabbitMQ-ready)
DB connection pooling
Stateless API → horizontally scalable
🔒 Security
bcrypt password hashing
JWT authentication
RBAC middleware
Rate limiting (100 req/min)
Input validation
🐳 Docker Services
API
PostgreSQL
Prometheus
Grafana
RabbitMQ (optional future)
🚧 Future Improvements
Switch to RabbitMQ/Kafka
WebSocket real-time updates
AI prompt tuning
Multi-language support
👨‍💻 Author

Shashank Gusain
Backend Engineer | Golang | Distributed Systems

⭐ Final Note

This project demonstrates:

Real-world backend architecture
Async processing with workers
AI integration
Production-level practices
