package database

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/gorm"
)

// txContextKey is a private type used as the context key for storing transactions.
// Using a private type prevents collisions with keys from other packages.
type txContextKey struct{}

// TransactionManager defines the interface for managing database transactions
// in the service layer following clean architecture principles.
type TransactionManager interface {
	// WithTransaction executes fn within a database transaction.
	// If a transaction already exists in the context (nested call), fn runs
	// with the existing transaction — no new transaction is started.
	// On success the transaction is committed; on error or panic it is rolled back.
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// txManager implements TransactionManager using GORM.
type txManager struct {
	db *gorm.DB
}

// NewTransactionManager creates a new TransactionManager backed by the given GORM DB.
func NewTransactionManager(db *gorm.DB) TransactionManager {
	return &txManager{db: db}
}

// WithTransaction starts a new database transaction, stores it in the context,
// and executes fn. It handles commit, rollback, and panic recovery.
// If a transaction already exists in ctx (nested call), fn is executed directly
// with the existing context to prevent double-begin.
func (m *txManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// Nested transaction detection: if a tx already exists in context, just run fn.
	if _, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok {
		slog.DebugContext(ctx, "nested transaction detected, reusing existing transaction")
		return fn(ctx)
	}

	// Begin a new transaction.
	tx := m.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Store the transaction in context so repositories can pick it up.
	txCtx := context.WithValue(ctx, txContextKey{}, tx)

	// Ensure rollback on panic, then re-panic.
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "panic in transaction, rolling back", "panic", r)
			tx.Rollback()
			panic(r)
		}
	}()

	// Execute the transactional function.
	if err := fn(txCtx); err != nil {
		slog.DebugContext(ctx, "transaction function returned error, rolling back", "error", err)
		if rbErr := tx.Rollback().Error; rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	// Commit the transaction.
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetTxFromContext extracts the GORM transaction stored in the context.
// Returns nil if no transaction is present.
func GetTxFromContext(ctx context.Context) *gorm.DB {
	tx, _ := ctx.Value(txContextKey{}).(*gorm.DB)
	return tx
}

// GetDB returns the transaction from context if available, otherwise returns
// the provided fallback db. This is the primary helper for repositories to
// transparently participate in transactions.
func GetDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx := GetTxFromContext(ctx); tx != nil {
		return tx
	}
	return db.WithContext(ctx)
}
