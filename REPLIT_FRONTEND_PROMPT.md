# InterviewAI — Frontend (Replit Prompt)

## Project Overview
Build a production-grade frontend for an AI-powered technical interview platform. The backend is a Go (Gin) REST API running at `http://localhost:8081`. Auth is JWT-based (HS256). Use **React + Vite + TailwindCSS**. No UI library — build components from scratch.

---

## Tech Stack
- React 18 + Vite
- TailwindCSS
- React Router v6
- Axios (for API calls)
- Context API (for auth state)

---

## Backend Base URL
```
http://localhost:8081
```
All authenticated requests must include the header:
```
Authorization: Bearer <jwt_token>
```

---

## Auth Flow
- Token is returned on register/login as `{ token, user }`.
- Store token in `localStorage` as `authToken`.
- Store user object as `authUser` (JSON string).
- On app load, read from localStorage and hydrate auth context.
- If token is missing/expired, redirect to `/login`.

---

## Roles
| Role | Access |
|------|--------|
| `admin` | Everything |
| `recruiter` | Create/manage interviews, view analytics |
| `candidate` | Take interviews, submit answers, view own results |

Route guards must enforce role-based access.

---

## API Reference

### Auth

#### POST /api/auth/register
```json
// Request
{ "name": "Alice", "email": "alice@test.com", "password": "password123", "role": "candidate" }

// Response 201
{ "token": "<jwt>", "user": { "id": "uuid", "name": "Alice", "email": "alice@test.com", "role": "candidate", "createdAt": "..." } }
```

#### POST /api/auth/login
```json
// Request
{ "email": "alice@test.com", "password": "password123" }

// Response 200
{ "token": "<jwt>", "user": { "id": "uuid", "name": "Alice", "email": "alice@test.com", "role": "candidate", "createdAt": "..." } }
```

#### GET /api/auth/me  [AUTH REQUIRED]
```json
// Response 200
{ "id": "uuid", "name": "Alice", "email": "alice@test.com", "role": "candidate", "createdAt": "..." }
```

---

### Interviews

#### GET /api/interviews  [AUTH REQUIRED]
Query params: `status` (active|draft|closed), `page` (default 1), `limit` (default 20)
```json
// Response 200
{
  "interviews": [
    { "id": "uuid", "title": "Go Backend", "description": "...", "status": "active", "createdBy": "uuid", "durationMinutes": 60, "createdAt": "...", "updatedAt": "..." }
  ],
  "total": 10,
  "page": 1,
  "limit": 20
}
```

#### POST /api/interviews  [RECRUITER/ADMIN]
```json
// Request
{ "title": "Go Backend Interview", "description": "Senior assessment", "status": "active", "durationMinutes": 60 }

// Response 201
{ "id": "uuid", "title": "...", "status": "active", "createdBy": "uuid", "durationMinutes": 60, "createdAt": "...", "updatedAt": "..." }
```

#### GET /api/interviews/:id  [AUTH REQUIRED]
```json
// Response 200
{
  "id": "uuid", "title": "...", "status": "active", "durationMinutes": 60, "createdBy": "uuid",
  "questions": [
    { "id": "uuid", "interviewId": "uuid", "text": "Explain goroutines", "type": "text", "order": 1, "maxScore": 100, "expectedAnswer": null, "createdAt": "..." }
  ]
}
```

#### PUT /api/interviews/:id  [RECRUITER/ADMIN]
```json
// Request (all fields optional)
{ "title": "Updated Title", "status": "closed", "durationMinutes": 90, "description": "Updated desc" }

// Response 200 — updated interview object
```

---

### Questions

#### POST /api/interviews/:id/questions  [RECRUITER/ADMIN]
```json
// Request
{ "text": "Explain goroutines vs OS threads", "type": "text", "maxScore": 100, "order": 1, "expectedAnswer": "Goroutines use M:N scheduling..." }

// type must be one of: text | coding | behavioral
// Response 201
{ "id": "uuid", "interviewId": "uuid", "text": "...", "type": "text", "order": 1, "maxScore": 100, "createdAt": "..." }
```

---

### Candidate Flow

#### POST /api/interview/start  [CANDIDATE]
```json
// Request
{ "interviewId": "uuid" }

// Response 200
{
  "sessionId": "sess_abc123",
  "interviewId": "uuid",
  "candidateId": "uuid",
  "startedAt": "...",
  "questions": [ { "id": "uuid", "text": "...", "type": "text", "order": 1, "maxScore": 100 } ]
}
```

#### POST /api/answer/submit  [CANDIDATE]
```json
// Request
{ "questionId": "uuid", "interviewId": "uuid", "answerText": "My answer...", "sessionId": "sess_abc123" }

// Response 202
{ "answerId": "uuid", "status": "queued", "message": "Answer submitted and queued for AI evaluation" }
```

#### GET /api/answers/:id  [AUTH REQUIRED]
```json
// Response 200 — while processing
{ "id": "uuid", "questionId": "uuid", "interviewId": "uuid", "candidateId": "uuid", "answerText": "...", "status": "queued|processing|completed|failed", "submittedAt": "..." }

// Response 200 — when completed (status = "completed")
{
  "id": "uuid", "status": "completed",
  "evaluation": {
    "id": "uuid", "answerId": "uuid", "score": 85.0, "feedback": "Good explanation...",
    "strengths": ["Clear explanation", "Good examples"],
    "areasOfImprovement": ["Missing edge cases"],
    "retryCount": 0, "evaluatedAt": "..."
  }
}
```
Poll this endpoint every 3 seconds until `status === "completed"` or `"failed"`.

---

### Results & Analytics

#### GET /api/results/:candidateId  [AUTH REQUIRED]
Query params: `interviewId` (optional)
```json
// Response 200
{
  "candidate": { "id": "uuid", "name": "Alice", "email": "...", "role": "candidate", "createdAt": "..." },
  "totalScore": 250.0,
  "averageScore": 83.3,
  "answersEvaluated": 3,
  "results": [
    {
      "answer": { "id": "uuid", "questionId": "uuid", "answerText": "...", "status": "completed" },
      "question": { "id": "uuid", "text": "Explain goroutines", "type": "text", "maxScore": 100 },
      "evaluation": { "score": 85, "feedback": "...", "strengths": [], "areasOfImprovement": [] }
    }
  ]
}
```

#### GET /api/evaluations/:answerId  [AUTH REQUIRED]
```json
// Response 200
{ "id": "uuid", "answerId": "uuid", "score": 85.0, "feedback": "...", "strengths": [...], "areasOfImprovement": [...], "evaluatedAt": "..." }
```

#### GET /api/evaluations/interview/:interviewId/leaderboard  [AUTH REQUIRED]
```json
// Response 200
{
  "interviewId": "uuid",
  "entries": [
    {
      "rank": 1,
      "candidate": { "id": "uuid", "name": "Alice", "email": "...", "role": "candidate" },
      "totalScore": 280.0,
      "averageScore": 93.3,
      "answersEvaluated": 3
    }
  ]
}
```

#### GET /api/analytics/overview  [RECRUITER/ADMIN]
```json
// Response 200
{
  "totalInterviews": 5,
  "activeInterviews": 3,
  "totalCandidates": 12,
  "totalAnswersSubmitted": 48,
  "totalAnswersEvaluated": 45,
  "pendingEvaluations": 3,
  "averageScore": 76.4,
  "topPerformers": []
}
```

---

### System

#### GET /health  [PUBLIC]
```json
{ "status": "ok" }
```

#### GET /api/users  [ADMIN ONLY]
Query params: `role` (optional)
```json
// Response 200
{
  "users": [ { "id": "uuid", "name": "Alice", "email": "...", "role": "candidate", "createdAt": "..." } ],
  "total": 12
}
```

---

## Pages & Routes

### Public Routes
| Path | Component | Description |
|------|-----------|-------------|
| `/login` | `LoginPage` | Email + password login form |
| `/register` | `RegisterPage` | Name, email, password, role selector |

### Protected Routes (all roles)
| Path | Component | Description |
|------|-----------|-------------|
| `/` | `DashboardPage` | Role-aware home — redirects to role dashboard |
| `/interviews` | `InterviewListPage` | Paginated list, filter by status |
| `/interviews/:id` | `InterviewDetailPage` | Interview info + questions list |

### Recruiter/Admin Routes
| Path | Component | Description |
|------|-----------|-------------|
| `/interviews/new` | `CreateInterviewPage` | Form to create interview |
| `/interviews/:id/edit` | `EditInterviewPage` | Edit title, status, duration |
| `/interviews/:id/questions/add` | `AddQuestionPage` | Add question to interview |
| `/interviews/:id/leaderboard` | `LeaderboardPage` | Ranked candidates for interview |
| `/analytics` | `AnalyticsPage` | Overview stats dashboard |
| `/users` | `UsersPage` | Admin only — list all users |

### Candidate Routes
| Path | Component | Description |
|------|-----------|-------------|
| `/interview/:id/take` | `TakeInterviewPage` | Start interview → answer questions one by one |
| `/results/:candidateId` | `ResultsPage` | Full scorecard with AI evaluations |

---

## Key UI Behaviors

### TakeInterviewPage (most important)
1. Call `POST /api/interview/start` with `interviewId` on mount.
2. Show questions one at a time (step 1 of N).
3. Each question shows: question text, type badge, max score, a textarea for the answer.
4. On "Submit Answer" click → call `POST /api/answer/submit`.
5. Show a loading spinner and poll `GET /api/answers/:id` every 3s.
6. When `status === "completed"`, show the evaluation card inline:
   - Score (large, color-coded: green ≥80, amber 60-79, red <60)
   - Score bar
   - Feedback text
   - Strengths list (green checkmarks)
   - Areas of Improvement list (amber arrows)
7. After evaluation shown, allow moving to the next question.
8. After all questions answered, show a summary with link to full results.

### Auth Context
```js
// AuthContext provides:
{ user, token, login(token, user), logout(), isAuthenticated, hasRole(role) }
```

### Axios instance
```js
// src/api/axios.js
import axios from 'axios'
const api = axios.create({ baseURL: 'http://localhost:8081' })
api.interceptors.request.use(config => {
  const token = localStorage.getItem('authToken')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})
api.interceptors.response.use(
  r => r,
  err => {
    if (err.response?.status === 401) {
      localStorage.removeItem('authToken')
      localStorage.removeItem('authUser')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)
export default api
```

---

## Project File Structure
```
src/
├── api/
│   ├── axios.js          # configured axios instance
│   ├── auth.js           # register, login, getMe
│   ├── interviews.js     # list, get, create, update, start
│   ├── questions.js      # addQuestion
│   ├── answers.js        # submitAnswer, getAnswer
│   ├── evaluations.js    # getEvaluation, getResults, getLeaderboard
│   └── analytics.js      # getOverview, listUsers
├── context/
│   └── AuthContext.jsx   # AuthProvider, useAuth
├── components/
│   ├── ProtectedRoute.jsx
│   ├── RoleRoute.jsx
│   ├── Navbar.jsx
│   ├── ScoreCard.jsx     # reusable eval score display
│   └── StatusBadge.jsx   # active/draft/closed pill
├── pages/
│   ├── LoginPage.jsx
│   ├── RegisterPage.jsx
│   ├── DashboardPage.jsx
│   ├── InterviewListPage.jsx
│   ├── InterviewDetailPage.jsx
│   ├── CreateInterviewPage.jsx
│   ├── EditInterviewPage.jsx
│   ├── AddQuestionPage.jsx
│   ├── TakeInterviewPage.jsx
│   ├── ResultsPage.jsx
│   ├── LeaderboardPage.jsx
│   ├── AnalyticsPage.jsx
│   └── UsersPage.jsx
└── main.jsx
```

---

## Design System (Tailwind)
- Background: `gray-950`, Cards: `gray-900`, Borders: `gray-800`
- Primary: `blue-500`, Success: `green-500`, Warning: `amber-500`, Error: `red-500`
- All forms: dark input style `bg-gray-800 border-gray-700 text-white focus:border-blue-500`
- Buttons: `bg-blue-600 hover:bg-blue-700 text-white font-semibold rounded-lg px-4 py-2`
- Score color logic: score >= 80 → green, score >= 60 → amber, else → red

---

## Error Handling
- Show inline error messages below forms on 4xx responses.
- Show a global toast/snackbar for 5xx errors.
- Empty states: show a message + CTA button when lists are empty.

---

## Notes
- CORS is already enabled on the backend (`Access-Control-Allow-Origin: *`).
- The backend runs on port `8081`. Vite dev server runs on `5173`.
- No HTTPS needed for local dev.
- Role `admin` can do everything `recruiter` can, plus `/api/users`.
- Password minimum length is 6 characters (backend enforces this).
