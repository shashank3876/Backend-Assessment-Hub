package config

import (
	"encoding/json"
	"testing"

	"github.com/pion/webrtc/v4"
)

func TestParseICEServersJSON_Default(t *testing.T) {
	servers, err := parseICEServersJSON("")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 default ICE server, got %d", len(servers))
	}
	if len(servers[0].URLs) < 1 || servers[0].URLs[0] == "" {
		t.Fatalf("expected default STUN url")
	}
}

func TestParseICEServersJSON_Custom(t *testing.T) {
	raw := `[{"urls":["stun:stun.l.google.com:19302"]}]`
	servers, err := parseICEServersJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].URLs[0] != "stun:stun.l.google.com:19302" {
		t.Fatalf("unexpected url: %v", servers[0].URLs)
	}
}

func TestParseICEServersJSON_TURNPassword(t *testing.T) {
	raw := `[{"urls":["turn:turn.example.com:3478"],"username":"u","credential":"p","credentialType":"password"}]`
	var servers []webrtc.ICEServer
	if err := json.Unmarshal([]byte(raw), &servers); err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatal("expected 1 server")
	}
}
