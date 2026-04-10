package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/saas-payment-platform/backend/internal/pkg/logger"
)

// captureAuditLog sets up a buffer-backed logger and returns the buffer for inspection.
func captureAuditLog() *bytes.Buffer {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})
	SetLogger(slog.New(handler).With("layer", "audit"))
	return &buf
}

func TestLog_IncludesAllRequiredFields(t *testing.T) {
	buf := captureAuditLog()

	ctx := context.Background()
	Log(ctx, Entry{
		UserID:   "user-123",
		Action:   ActionLogin,
		Endpoint: "/api/v1/auth/login",
		Result:   ResultSuccess,
	})

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	if record["user_id"] != "user-123" {
		t.Errorf("expected user_id=user-123, got %v", record["user_id"])
	}
	if record["audit_action"] != "user.login" {
		t.Errorf("expected audit_action=user.login, got %v", record["audit_action"])
	}
	if record["endpoint"] != "/api/v1/auth/login" {
		t.Errorf("expected endpoint=/api/v1/auth/login, got %v", record["endpoint"])
	}
	if record["result"] != "success" {
		t.Errorf("expected result=success, got %v", record["result"])
	}
	if record["audit_timestamp"] == nil {
		t.Error("expected audit_timestamp to be present")
	}
	if record["layer"] != "audit" {
		t.Errorf("expected layer=audit, got %v", record["layer"])
	}
}

func TestLog_IncludesOptionalFields(t *testing.T) {
	buf := captureAuditLog()

	ctx := context.Background()
	Log(ctx, Entry{
		UserID:   "user-456",
		Action:   ActionAPIKeyCreate,
		Resource: "key-789",
		Endpoint: "/api/v1/api-keys",
		Result:   ResultSuccess,
		Detail:   "created new API key",
	})

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	if record["resource"] != "key-789" {
		t.Errorf("expected resource=key-789, got %v", record["resource"])
	}
	if record["detail"] != "created new API key" {
		t.Errorf("expected detail='created new API key', got %v", record["detail"])
	}
}

func TestLog_ExtractsRequestIDFromContext(t *testing.T) {
	buf := captureAuditLog()

	ctx := context.WithValue(context.Background(), logger.RequestIDKey, "req-abc-123")
	LogSuccess(ctx, ActionLogin, "user-123", "/api/v1/auth/login")

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	if record["request_id"] != "req-abc-123" {
		t.Errorf("expected request_id=req-abc-123, got %v", record["request_id"])
	}
}

func TestLogSuccess_SetsResultSuccess(t *testing.T) {
	buf := captureAuditLog()

	LogSuccess(context.Background(), ActionRegister, "user-new", "/api/v1/auth/register")

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	if record["result"] != "success" {
		t.Errorf("expected result=success, got %v", record["result"])
	}
	if record["audit_action"] != "user.register" {
		t.Errorf("expected audit_action=user.register, got %v", record["audit_action"])
	}
}

func TestLogFailure_SetsResultFailureWithDetail(t *testing.T) {
	buf := captureAuditLog()

	LogFailure(context.Background(), ActionLogin, "user-bad", "/api/v1/auth/login", "invalid credentials")

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	if record["result"] != "failure" {
		t.Errorf("expected result=failure, got %v", record["result"])
	}
	if record["detail"] != "invalid credentials" {
		t.Errorf("expected detail='invalid credentials', got %v", record["detail"])
	}
}

func TestLogWithResource_IncludesResourceID(t *testing.T) {
	buf := captureAuditLog()

	LogWithResource(context.Background(), ActionAPIKeyRevoke, "user-123", "/api/v1/api-keys/key-1", "key-1", ResultSuccess)

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	if record["resource"] != "key-1" {
		t.Errorf("expected resource=key-1, got %v", record["resource"])
	}
	if record["audit_action"] != "apikey.revoke" {
		t.Errorf("expected audit_action=apikey.revoke, got %v", record["audit_action"])
	}
}

func TestLog_SeparateFromApplicationLogs(t *testing.T) {
	buf := captureAuditLog()

	LogSuccess(context.Background(), ActionAdminAccess, "admin-1", "/api/v1/admin/stats")

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to parse audit log JSON: %v", err)
	}

	// The "layer":"audit" attribute distinguishes audit logs from app logs.
	if record["layer"] != "audit" {
		t.Errorf("expected layer=audit for separation from app logs, got %v", record["layer"])
	}
}
