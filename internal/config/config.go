package config

import (
	"log"
	"os"
)

type Config struct {
	Port              string
	DatabaseURL       string
	JWTSecret         string
	OpenAIBaseURL     string
	OpenAIAPIKey      string
	WorkerConcurrency int
	RateLimit         int
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "interview-eval-secret-change-in-production"
	}

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		JWTSecret:         jwtSecret,
		OpenAIBaseURL:     os.Getenv("AI_INTEGRATIONS_OPENAI_BASE_URL"),
		OpenAIAPIKey:      os.Getenv("AI_INTEGRATIONS_OPENAI_API_KEY"),
		WorkerConcurrency: 5,
		RateLimit:         100,
	}
}
