// Package audit provides a dedicated audit logger for security-sensitive operations.
// Audit logs are written separately from application logs using a distinct slog.Logger
// instance with structured JSON output and an "audit" prefix for easy filtering.
//
// Each audit entry includes: who (user_id), what (action), when (timestamp),
// where (endpoint/service), and result (success/failure).
package audit

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/saas-payment-platform/backend/internal/pkg/logger"
)

// Action represents a security-sensitive operation being audited.
type Action string

const (
	ActionLogin              Action = "user.login"
	ActionLogout             Action = "user.logout"
	ActionRegister           Action = "user.register"
	ActionAPIKeyCreate       Action = "apikey.create"
	ActionAPIKeyRevoke       Action = "apikey.revoke"
	ActionTransactionCreate  Action = "transaction.create"
	ActionTransactionUpdate  Action = "transaction.status_change"
	ActionWebhookCreate      Action = "webhook.create"
	ActionWebhookDelete      Action = "webhook.delete"
	ActionSubscriptionCreate Action = "subscription.create"
	ActionSubscriptionCancel Action = "subscription.cancel"
	ActionAdminAccess        Action = "admin.access"
)

// Result represents the outcome of an audited operation.
type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
)

// Entry holds all fields for a single audit log record.
type Entry struct {
	UserID   string // who — the user performing the action
	Action   Action // what — the operation being performed
	Resource string // optional resource identifier (e.g., API key ID, transaction ID)
	Endpoint string // where — the API endpoint or service name
	Result   Result // outcome — success or failure
	Detail   string // optional additional context
}

// auditLogger is a separate slog.Logger dedicated to audit logs.
// It writes to stderr with a distinct "layer":"audit" attribute
// so audit entries can be filtered/routed independently from app logs.
var auditLogger *slog.Logger

func init() {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	auditLogger = slog.New(handler).With("layer", "audit")
}

// SetLogger allows overriding the audit logger (useful for testing).
func SetLogger(l *slog.Logger) {
	auditLogger = l
}

// Log writes an audit log entry with all required fields.
// It extracts request_id from context if available.
func Log(ctx context.Context, entry Entry) {
	attrs := []slog.Attr{
		slog.String("audit_action", string(entry.Action)),
		slog.String("user_id", entry.UserID),
		slog.String("endpoint", entry.Endpoint),
		slog.String("result", string(entry.Result)),
		slog.Time("audit_timestamp", time.Now().UTC()),
	}

	if entry.Resource != "" {
		attrs = append(attrs, slog.String("resource", entry.Resource))
	}
	if entry.Detail != "" {
		attrs = append(attrs, slog.String("detail", entry.Detail))
	}

	// Extract request_id from context for correlation.
	if reqID, ok := ctx.Value(logger.RequestIDKey).(string); ok && reqID != "" {
		attrs = append(attrs, slog.String("request_id", reqID))
	}

	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}

	auditLogger.Info("AUDIT", args...)
}

// LogSuccess is a convenience wrapper for successful operations.
func LogSuccess(ctx context.Context, action Action, userID, endpoint string) {
	Log(ctx, Entry{
		UserID:   userID,
		Action:   action,
		Endpoint: endpoint,
		Result:   ResultSuccess,
	})
}

// LogFailure is a convenience wrapper for failed operations.
func LogFailure(ctx context.Context, action Action, userID, endpoint, detail string) {
	Log(ctx, Entry{
		UserID:   userID,
		Action:   action,
		Endpoint: endpoint,
		Result:   ResultFailure,
		Detail:   detail,
	})
}

// LogWithResource is a convenience wrapper that includes a resource identifier.
func LogWithResource(ctx context.Context, action Action, userID, endpoint, resource string, result Result) {
	Log(ctx, Entry{
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Endpoint: endpoint,
		Result:   result,
	})
}
