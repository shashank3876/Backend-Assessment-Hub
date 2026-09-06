package sfu

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/interview-eval/backend/internal/config"
	"github.com/interview-eval/backend/internal/models"
	"github.com/pion/webrtc/v4"
)

// ErrSlotTaken is returned when a second participant tries to use the same role slot.
var ErrSlotTaken = errors.New("participant slot already filled")

// Hub owns SFU rooms keyed by interview session ID (from StartInterview).
type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
	api   *webrtc.API
	ice   []webrtc.ICEServer
}

// NewHub builds the SFU with optional UDP port range from config.
func NewHub(cfg *config.Config) *Hub {
	s := webrtc.SettingEngine{}
	if cfg.WebRTCUDPPortMin > 0 && cfg.WebRTCUDPPortMax > 0 {
		_ = s.SetEphemeralUDPPortRange(uint16(cfg.WebRTCUDPPortMin), uint16(cfg.WebRTCUDPPortMax))
	}
	api := webrtc.NewAPI(webrtc.WithSettingEngine(s))
	ice := append([]webrtc.ICEServer(nil), cfg.WebRTCICEServers...)
	return &Hub{
		rooms: make(map[string]*Room),
		api:   api,
		ice:   ice,
	}
}

// GetOrCreateRoom returns the room for a session, creating it if needed.
func (h *Hub) GetOrCreateRoom(sessionID string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[sessionID]; ok {
		return r
	}
	r := newRoom(sessionID, h, h.api, h.ice)
	h.rooms[sessionID] = r
	return r
}

func (h *Hub) removeRoom(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, sessionID)
}

// Room is a 1:1 SFU session (candidate + interviewer).
type Room struct {
	mu   sync.Mutex
	cond *sync.Cond
	id   string
	hub  *Hub
	api  *webrtc.API
	ice  []webrtc.ICEServer

	Candidate *Participant
	Recruiter *Participant
}

func newRoom(id string, hub *Hub, api *webrtc.API, ice []webrtc.ICEServer) *Room {
	r := &Room{id: id, hub: hub, api: api, ice: append([]webrtc.ICEServer(nil), ice...)}
	r.cond = sync.NewCond(&r.mu)
	return r
}

// Participant is one browser connection to the SFU.
type Participant struct {
	UserID  string
	Role    models.UserRole
	room    *Room
	conn    *websocket.Conn
	writeMu sync.Mutex
	mu      sync.Mutex
	pc      *webrtc.PeerConnection
	pending []*webrtc.ICECandidateInit
}

// SignalMessage is exchanged over WebSocket for WebRTC signaling.
type SignalMessage struct {
	Type       string             `json:"type"`
	SDP        string             `json:"sdp,omitempty"`
	Candidate  json.RawMessage    `json:"candidate,omitempty"`
	Message    string             `json:"message,omitempty"`
	IceServers []webrtc.ICEServer `json:"iceServers,omitempty"`
	SessionID  string             `json:"sessionId,omitempty"`
	Role       string             `json:"role,omitempty"`
}

// TryRegister reserves the candidate or recruiter slot for this connection.
func (r *Room) TryRegister(conn *websocket.Conn, userID string, role models.UserRole) (*Participant, error) {
	p := &Participant{UserID: userID, Role: role, room: r, conn: conn}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch role {
	case models.RoleCandidate:
		if r.Candidate != nil {
			return nil, ErrSlotTaken
		}
		r.Candidate = p
	case models.RoleRecruiter, models.RoleAdmin:
		if r.Recruiter != nil {
			return nil, ErrSlotTaken
		}
		r.Recruiter = p
	default:
		return nil, fmt.Errorf("unsupported role for WebRTC")
	}
	r.cond.Broadcast()
	return p, nil
}

func (r *Room) other(self *Participant) *Participant {
	switch self.Role {
	case models.RoleCandidate:
		return r.Recruiter
	case models.RoleRecruiter, models.RoleAdmin:
		return r.Candidate
	default:
		return nil
	}
}

func (r *Room) waitOtherPeer(self *Participant) *Participant {
	r.mu.Lock()
	defer r.mu.Unlock()
	for {
		other := r.other(self)
		if other != nil {
			other.mu.Lock()
			hasPC := other.pc != nil
			other.mu.Unlock()
			if hasPC {
				return other
			}
		}
		// Check if self is still in the room before waiting
		self.mu.Lock()
		active := self.pc != nil
		self.mu.Unlock()
		if !active {
			return nil
		}
		r.cond.Wait()
	}
}

func (r *Room) forwardTrack(from *Participant, track *webrtc.TrackRemote) {
	other := r.waitOtherPeer(from)
	if other == nil {
		return
	}
	localTrack, err := webrtc.NewTrackLocalStaticRTP(
		track.Codec().RTPCodecCapability,
		track.ID(),
		track.StreamID(),
	)
	if err != nil {
		return
	}
	other.mu.Lock()
	pc := other.pc
	other.mu.Unlock()
	if pc == nil {
		return
	}
	sender, err := pc.AddTrack(localTrack)
	if err != nil {
		return
	}

	// Trigger renegotiation so the other peer gets the new track
	offer, err := pc.CreateOffer(nil)
	if err == nil {
		if err := pc.SetLocalDescription(offer); err == nil {
			_ = other.SendSignal(&SignalMessage{Type: "offer", SDP: pc.LocalDescription().SDP})
		}
	}

	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, err := sender.Read(rtcpBuf); err != nil {
				return
			}
		}
	}()
	for {
		rtpPkt, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		if err := localTrack.WriteRTP(rtpPkt); err != nil {
			return
		}
	}
}

// HandleOffer applies the browser offer and returns the SFU answer SDP.
func (r *Room) HandleOffer(p *Participant, offerSDP string) (*webrtc.SessionDescription, error) {
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}
	config := webrtc.Configuration{ICEServers: r.ice}
	pc, err := r.api.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	if p.pc != nil {
		_ = p.pc.Close()
	}
	p.pc = pc
	pending := p.pending
	p.pending = nil
	p.mu.Unlock()

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		j := c.ToJSON()
		b, err := json.Marshal(j)
		if err != nil {
			return
		}
		_ = p.SendSignal(&SignalMessage{Type: "ice-candidate", Candidate: b})
	})

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go r.forwardTrack(p, track)
	})

	if err := pc.SetRemoteDescription(offer); err != nil {
		return nil, err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		return nil, err
	}
	for _, c := range pending {
		_ = pc.AddICECandidate(*c)
	}
	r.mu.Lock()
	r.cond.Broadcast()
	r.mu.Unlock()
	return pc.LocalDescription(), nil
}

// HandleAnswer applies the browser answer SDP during renegotiation.
func (r *Room) HandleAnswer(p *Participant, answerSDP string) error {
	p.mu.Lock()
	pc := p.pc
	p.mu.Unlock()
	if pc == nil {
		return nil
	}
	return pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answerSDP})
}

// AddICECandidate adds a trickle ICE candidate from the browser.
func (r *Room) AddICECandidate(p *Participant, init *webrtc.ICECandidateInit) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pc == nil {
		initCopy := *init
		p.pending = append(p.pending, &initCopy)
		return nil
	}
	return p.pc.AddICECandidate(*init)
}

// SendSignal writes a signaling message to the browser WebSocket (serialized JSON).
func (p *Participant) SendSignal(msg *SignalMessage) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.conn.WriteJSON(msg)
}

// RemoveParticipant tears down the peer connection and frees the slot.
func (r *Room) RemoveParticipant(p *Participant) {
	r.mu.Lock()
	if r.Candidate == p {
		r.Candidate = nil
	}
	if r.Recruiter == p {
		r.Recruiter = nil
	}
	empty := r.Candidate == nil && r.Recruiter == nil
	r.mu.Unlock()

	p.mu.Lock()
	if p.pc != nil {
		_ = p.pc.Close()
		p.pc = nil
	}
	p.mu.Unlock()

	r.mu.Lock()
	r.cond.Broadcast()
	r.mu.Unlock()

	if empty {
		r.hub.removeRoom(r.id)
	}
}
