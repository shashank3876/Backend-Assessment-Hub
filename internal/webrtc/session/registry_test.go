package session

import "testing"

func TestRegistry_RegisterGet(t *testing.T) {
	r := NewRegistry()
	r.Register("sess_1", "iv-a", "user-cand")
	info, ok := r.Get("sess_1")
	if !ok {
		t.Fatal("expected session")
	}
	if info.InterviewID != "iv-a" || info.CandidateID != "user-cand" {
		t.Fatalf("unexpected info: %+v", info)
	}
	r.Delete("sess_1")
	if _, ok := r.Get("sess_1"); ok {
		t.Fatal("expected session removed")
	}
}
