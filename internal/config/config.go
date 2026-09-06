package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	JWTSecret              string
	ClerkJWKSURL           string
	OpenAIBaseURL          string
	OpenAIAPIKey           string
	WorkerConcurrency      int
	RedisAddr              string
	RedisPassword          string
	RedisDB                int
	RedisQueueKey          string
	QueueBlockTimeout      time.Duration
	EvaluationCacheTTL     time.Duration
	RateLimitUserPerMinute int
	WebRTCICEServers       []webrtc.ICEServer
	WebRTCUDPPortMin       int
	WebRTCUDPPortMax       int
}

func Load() *Config {
	port := os.Getenv("GO_PORT")
	if port == "" {
		port = "8081"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	clerkJWKSURL := os.Getenv("CLERK_JWKS_URL")

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		log.Fatal("REDIS_ADDR environment variable is required")
	}

	redisDB := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("REDIS_DB must be an integer: %v", err)
		}
		redisDB = n
	}

	queueKey := os.Getenv("REDIS_QUEUE_KEY")
	if queueKey == "" {
		queueKey = "eval:jobs:queue"
	}

	blockSec := 5
	if v := os.Getenv("REDIS_QUEUE_BLOCK_SECONDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("REDIS_QUEUE_BLOCK_SECONDS must be an integer: %v", err)
		}
		if n > 0 {
			blockSec = n
		}
	}

	cacheTTLMin := 10
	if v := os.Getenv("EVALUATION_CACHE_TTL_MINUTES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("EVALUATION_CACHE_TTL_MINUTES must be an integer: %v", err)
		}
		if n > 0 {
			cacheTTLMin = n
		}
	}

	rateUser := 100
	if v := os.Getenv("RATE_LIMIT_USER_PER_MINUTE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("RATE_LIMIT_USER_PER_MINUTE must be an integer: %v", err)
		}
		if n > 0 {
			rateUser = n
		}
	}

	workers := 5
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("WORKER_CONCURRENCY must be an integer: %v", err)
		}
		if n > 0 {
			workers = n
		}
	}

	iceServers, err := parseICEServersJSON(os.Getenv("WEBRTC_ICE_SERVERS_JSON"))
	if err != nil {
		log.Fatalf("WEBRTC_ICE_SERVERS_JSON: %v", err)
	}

	udpMin := 0
	if v := os.Getenv("WEBRTC_UDP_PORT_MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("WEBRTC_UDP_PORT_MIN must be an integer: %v", err)
		}
		udpMin = n
	}
	udpMax := 0
	if v := os.Getenv("WEBRTC_UDP_PORT_MAX"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("WEBRTC_UDP_PORT_MAX must be an integer: %v", err)
		}
		udpMax = n
	}

	return &Config{
		Port:                   port,
		DatabaseURL:            dbURL,
		JWTSecret:              os.Getenv("JWT_SECRET"),
		ClerkJWKSURL:           clerkJWKSURL,
		OpenAIBaseURL:          os.Getenv("AI_INTEGRATIONS_OPENAI_BASE_URL"),
		OpenAIAPIKey:           os.Getenv("AI_INTEGRATIONS_OPENAI_API_KEY"),
		WorkerConcurrency:      workers,
		RedisAddr:              redisAddr,
		RedisPassword:          os.Getenv("REDIS_PASSWORD"),
		RedisDB:                redisDB,
		RedisQueueKey:          queueKey,
		QueueBlockTimeout:      time.Duration(blockSec) * time.Second,
		EvaluationCacheTTL:     time.Duration(cacheTTLMin) * time.Minute,
		RateLimitUserPerMinute: rateUser,
		WebRTCICEServers:       iceServers,
		WebRTCUDPPortMin:       udpMin,
		WebRTCUDPPortMax:       udpMax,
	}
}

// parseICEServersJSON decodes a JSON array of ICE servers (Pion webrtc.ICEServer shape).
// Example: [{"urls":["stun:stun.l.google.com:19302"]}]
// If empty, returns a default public STUN server.
func parseICEServersJSON(raw string) ([]webrtc.ICEServer, error) {
	if strings.TrimSpace(raw) == "" {
		return []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}}, nil
	}
	var servers []webrtc.ICEServer
	if err := json.Unmarshal([]byte(raw), &servers); err != nil {
		return nil, err
	}
	return servers, nil
}
