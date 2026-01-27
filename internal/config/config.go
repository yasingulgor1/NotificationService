package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	App      AppConfig
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Webhook  WebhookConfig
	Worker   WorkerConfig
	Retry    RetryConfig
}

type AppConfig struct {
	Env      string
	LogLevel string
}

type ServerConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	URL          string
	MaxRetries   int
	PoolSize     int
	MinIdleConns int
}

type WebhookConfig struct {
	URL     string
	Timeout time.Duration
}

type WorkerConfig struct {
	SMSCount          int
	EmailCount        int
	PushCount         int
	RateLimitPerSec   int
	SchedulerInterval time.Duration
}

type RetryConfig struct {
	MaxCount  int
	BaseDelay time.Duration
}

// Load creates a new Config from environment variables
func Load() *Config {
	return &Config{
		App: AppConfig{
			Env:      getEnv("APP_ENV", "development"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		Server: ServerConfig{
			Port:            getEnv("SERVER_PORT", "8080"),
			ReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 15*time.Second),
			ShutdownTimeout: getDurationEnv("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			URL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/notifications?sslmode=disable"),
			MaxOpenConns:    getIntEnv("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getIntEnv("DATABASE_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDurationEnv("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			URL:          getEnv("REDIS_URL", "redis://localhost:6379/0"),
			MaxRetries:   getIntEnv("REDIS_MAX_RETRIES", 3),
			PoolSize:     getIntEnv("REDIS_POOL_SIZE", 10),
			MinIdleConns: getIntEnv("REDIS_MIN_IDLE_CONNS", 5),
		},
		Webhook: WebhookConfig{
			URL:     getEnv("WEBHOOK_URL", "https://webhook.site/test"),
			Timeout: getDurationEnv("WEBHOOK_TIMEOUT", 10*time.Second),
		},
		Worker: WorkerConfig{
			SMSCount:          getIntEnv("WORKER_COUNT_SMS", 5),
			EmailCount:        getIntEnv("WORKER_COUNT_EMAIL", 5),
			PushCount:         getIntEnv("WORKER_COUNT_PUSH", 5),
			RateLimitPerSec:   getIntEnv("RATE_LIMIT_PER_CHANNEL", 100),
			SchedulerInterval: getDurationEnv("SCHEDULER_INTERVAL", 10*time.Second),
		},
		Retry: RetryConfig{
			MaxCount:  getIntEnv("MAX_RETRY_COUNT", 5),
			BaseDelay: getDurationEnv("RETRY_BASE_DELAY", 1*time.Second),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
