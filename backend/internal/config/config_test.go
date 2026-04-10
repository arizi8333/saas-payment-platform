package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	envVars := []string{
		"SERVER_PORT", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSL_MODE",
		"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_MAX_LIFETIME",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB",
		"JWT_SECRET", "JWT_EXPIRATION_HOURS",
		"RATE_LIMIT_REQUESTS_PER_HOUR", "RATE_LIMIT_WINDOW_SIZE",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := Load()

	if cfg.Server.Port != "8080" {
		t.Errorf("expected Server.Port=8080, got %s", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Errorf("expected ReadTimeout=10s, got %v", cfg.Server.ReadTimeout)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("expected DB.Host=localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("expected MaxOpenConns=25, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Redis.Host != "localhost" {
		t.Errorf("expected Redis.Host=localhost, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.DB != 0 {
		t.Errorf("expected Redis.DB=0, got %d", cfg.Redis.DB)
	}
	if cfg.JWT.ExpirationHours != 24 {
		t.Errorf("expected JWT.ExpirationHours=24, got %d", cfg.JWT.ExpirationHours)
	}
	if cfg.RateLimit.RequestsPerHour != 1000 {
		t.Errorf("expected RateLimit.RequestsPerHour=1000, got %d", cfg.RateLimit.RequestsPerHour)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("DB_MAX_OPEN_CONNS", "50")
	os.Setenv("REDIS_DB", "2")
	os.Setenv("JWT_EXPIRATION_HOURS", "48")
	os.Setenv("RATE_LIMIT_REQUESTS_PER_HOUR", "500")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_MAX_OPEN_CONNS")
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("JWT_EXPIRATION_HOURS")
		os.Unsetenv("RATE_LIMIT_REQUESTS_PER_HOUR")
	}()

	cfg := Load()

	if cfg.Server.Port != "9090" {
		t.Errorf("expected Server.Port=9090, got %s", cfg.Server.Port)
	}
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("expected DB.Host=db.example.com, got %s", cfg.Database.Host)
	}
	if cfg.Database.MaxOpenConns != 50 {
		t.Errorf("expected MaxOpenConns=50, got %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Redis.DB != 2 {
		t.Errorf("expected Redis.DB=2, got %d", cfg.Redis.DB)
	}
	if cfg.JWT.ExpirationHours != 48 {
		t.Errorf("expected JWT.ExpirationHours=48, got %d", cfg.JWT.ExpirationHours)
	}
	if cfg.RateLimit.RequestsPerHour != 500 {
		t.Errorf("expected RateLimit.RequestsPerHour=500, got %d", cfg.RateLimit.RequestsPerHour)
	}
}

func TestLoadInvalidIntFallsBackToDefault(t *testing.T) {
	os.Setenv("DB_MAX_OPEN_CONNS", "not_a_number")
	defer os.Unsetenv("DB_MAX_OPEN_CONNS")

	cfg := Load()

	if cfg.Database.MaxOpenConns != 25 {
		t.Errorf("expected fallback MaxOpenConns=25, got %d", cfg.Database.MaxOpenConns)
	}
}

func TestLoadInvalidDurationFallsBackToDefault(t *testing.T) {
	os.Setenv("SERVER_READ_TIMEOUT", "invalid")
	defer os.Unsetenv("SERVER_READ_TIMEOUT")

	cfg := Load()

	if cfg.Server.ReadTimeout != 10*time.Second {
		t.Errorf("expected fallback ReadTimeout=10s, got %v", cfg.Server.ReadTimeout)
	}
}

func TestDatabaseDSN(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	if got := cfg.DSN(); got != expected {
		t.Errorf("expected DSN=%q, got %q", expected, got)
	}
}

func TestRedisAddr(t *testing.T) {
	cfg := &RedisConfig{Host: "redis.local", Port: "6380"}

	if got := cfg.Addr(); got != "redis.local:6380" {
		t.Errorf("expected Addr=redis.local:6380, got %s", got)
	}
}
