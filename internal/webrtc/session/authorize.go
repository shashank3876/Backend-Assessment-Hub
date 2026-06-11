package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/interview-eval/backend/internal/db"
	"github.com/interview-eval/backend/internal/models"
)

// AuthorizeJoin checks that the user may join the WebRTC room for this session and interview.
func AuthorizeJoin(ctx context.Context, database *db.DB, reg *Registry, sessionID, interviewID, userID string, role models.UserRole) error {
	iv, err := database.GetInterviewByID(ctx, interviewID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("interview not found")
		}
		return err
	}
	if iv.Status != models.StatusActive {
		return fmt.Errorf("interview is not active")
	}

	switch role {
	case models.RoleCandidate:
		info, ok := reg.Get(sessionID)
		if !ok {
			return fmt.Errorf("unknown session")
		}
		if info.InterviewID != interviewID {
			return fmt.Errorf("interview does not match session")
		}
		if info.CandidateID != userID {
			return fmt.Errorf("not the session candidate")
		}
		return nil

	case models.RoleRecruiter:
		info, ok := reg.Get(sessionID)
		if !ok {
			return fmt.Errorf("unknown session")
		}
		if info.InterviewID != interviewID {
			return fmt.Errorf("interview does not match session")
		}
		if iv.CreatedBy != userID {
			return fmt.Errorf("not the interview owner")
		}
		return nil

	case models.RoleAdmin:
		info, ok := reg.Get(sessionID)
		if !ok {
			return fmt.Errorf("unknown session")
		}
		if info.InterviewID != interviewID {
			return fmt.Errorf("interview does not match session")
		}
		return nil

	default:
		return fmt.Errorf("invalid role for video session")
	}
}
