package sfu

import (
	"testing"

	"github.com/gorilla/websocket"
	"github.com/interview-eval/backend/internal/config"
	"github.com/interview-eval/backend/internal/models"
)

func TestRoom_TryRegisterSlots(t *testing.T) {
	h := NewHub(&config.Config{})
	room := h.GetOrCreateRoom("sess_x")

	var c1, c2, c3 websocket.Conn
	p1, err := room.TryRegister(&c1, "u1", models.RoleCandidate)
	if err != nil {
		t.Fatal(err)
	}
	if p1.Role != models.RoleCandidate {
		t.Fatal("role mismatch")
	}
	if _, err := room.TryRegister(&c2, "u2", models.RoleCandidate); err != ErrSlotTaken {
		t.Fatalf("expected ErrSlotTaken, got %v", err)
	}
	r1, err := room.TryRegister(&c3, "u2", models.RoleRecruiter)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Role != models.RoleRecruiter {
		t.Fatal("expected recruiter")
	}
}
