package session

import (
	"sync"
)

// Info ties a live interview session to the candidate who started it.
type Info struct {
	InterviewID string
	CandidateID string
}

// Registry stores session metadata for WebRTC authorization (in-memory; matches SFU room scope).
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]Info
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]Info)}
}

func (r *Registry) Register(sessionID, interviewID, candidateID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sessionID] = Info{InterviewID: interviewID, CandidateID: candidateID}
}

func (r *Registry) Get(sessionID string) (Info, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.sessions[sessionID]
	return info, ok
}

// ListByInterview returns all active sessionIDs for a given interviewID.
func (r *Registry) ListByInterview(interviewID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var ids []string
	for sid, info := range r.sessions {
		if info.InterviewID == interviewID {
			ids = append(ids, sid)
		}
	}
	return ids
}

func (r *Registry) Delete(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
}
