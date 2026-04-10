package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host         string
	Port         string
	User         string
	Password     string
	DBName       string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerHour int
	WindowSize      time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getDurationEnv("SERVER_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: getDurationEnv("SERVER_WRITE_TIMEOUT", 10*time.Second),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnv("DB_PORT", "5432"),
			User:         getEnv("DB_USER", "postgres"),
			Password:     getEnv("DB_PASSWORD", ""),
			DBName:       getEnv("DB_NAME", "saas_payment"),
			SSLMode:      getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns: getIntEnv("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getIntEnv("DB_MAX_IDLE_CONNS", 10),
			MaxLifetime:  getDurationEnv("DB_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getIntEnv("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", ""),
			ExpirationHours: getIntEnv("JWT_EXPIRATION_HOURS", 24),
		},
		RateLimit: RateLimitConfig{
			RequestsPerHour: getIntEnv("RATE_LIMIT_REQUESTS_PER_HOUR", 1000),
			WindowSize:      getDurationEnv("RATE_LIMIT_WINDOW_SIZE", 1*time.Hour),
		},
	}
}

// DSN returns the PostgreSQL connection string.
func (d *DatabaseConfig) DSN() string {
	return "host=" + d.Host +
		" port=" + d.Port +
		" user=" + d.User +
		" password=" + d.Password +
		" dbname=" + d.DBName +
		" sslmode=" + d.SSLMode
}

// Addr returns the Redis address in host:port format.
func (r *RedisConfig) Addr() string {
	return r.Host + ":" + r.Port
}

// getEnv reads an environment variable or returns a default value.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getIntEnv reads an environment variable as int or returns a default value.
func getIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// getDurationEnv reads an environment variable as time.Duration or returns a default value.
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
