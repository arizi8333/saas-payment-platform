package database

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/saas-payment-platform/backend/internal/config"
)

func TestNewRedisClient_HostPort(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := &config.RedisConfig{
		Host:     mr.Host(),
		Port:     mr.Port(),
		Password: "",
		DB:       0,
	}

	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewRedisClient_URL(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := &config.RedisConfig{
		URL: "redis://" + mr.Addr(),
	}

	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewRedisClient_URLTakesPrecedence(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := &config.RedisConfig{
		URL:      "redis://" + mr.Addr(),
		Host:     "wrong-host",
		Port:     "9999",
		Password: "",
		DB:       0,
	}

	// URL should take precedence; if host/port were used, this would fail.
	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("expected URL to take precedence, got error: %v", err)
	}
	defer client.Close()
}

func TestNewRedisClient_InvalidURL(t *testing.T) {
	cfg := &config.RedisConfig{
		URL: "://invalid-url",
	}

	_, err := NewRedisClient(cfg)
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestNewRedisClient_UnreachableHost(t *testing.T) {
	cfg := &config.RedisConfig{
		Host: "192.0.2.1", // RFC 5737 TEST-NET, guaranteed unreachable
		Port: "6379",
	}

	_, err := NewRedisClient(cfg)
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestRedisHealthCheck_Healthy(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := &config.RedisConfig{
		Host: mr.Host(),
		Port: mr.Port(),
	}

	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer client.Close()

	if err := RedisHealthCheck(client); err != nil {
		t.Fatalf("expected healthy, got %v", err)
	}
}

func TestRedisHealthCheck_Unhealthy(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := &config.RedisConfig{
		Host: mr.Host(),
		Port: mr.Port(),
	}

	client, err := NewRedisClient(cfg)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Close miniredis to simulate an unhealthy server.
	mr.Close()

	if err := RedisHealthCheck(client); err == nil {
		t.Fatal("expected health check to fail after server closed")
	}
}
