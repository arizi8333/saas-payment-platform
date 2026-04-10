package database

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testItem is a simple model used for transaction tests.
type testItem struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"type:varchar(100)"`
}

// setupTestDB creates an in-memory SQLite database with the testItem table.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&testItem{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
	return db
}

func countItems(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	db.Model(&testItem{}).Count(&count)
	return count
}

func TestWithTransaction_CommitOnSuccess(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)

	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		tx := GetTxFromContext(ctx)
		return tx.Create(&testItem{Name: "item1"}).Error
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c := countItems(t, db); c != 1 {
		t.Errorf("expected 1 item after commit, got %d", c)
	}
}

func TestWithTransaction_RollbackOnError(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)

	testErr := errors.New("something went wrong")
	err := tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		tx := GetTxFromContext(ctx)
		if err := tx.Create(&testItem{Name: "item1"}).Error; err != nil {
			return err
		}
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Fatalf("expected testErr, got %v", err)
	}
	if c := countItems(t, db); c != 0 {
		t.Errorf("expected 0 items after rollback, got %d", c)
	}
}

func TestWithTransaction_RollbackOnPanic(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic to be re-raised")
		}
		if r != "boom" {
			t.Fatalf("expected panic value 'boom', got %v", r)
		}
		// After panic recovery, the item should NOT be persisted.
		if c := countItems(t, db); c != 0 {
			t.Errorf("expected 0 items after panic rollback, got %d", c)
		}
	}()

	_ = tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		tx := GetTxFromContext(ctx)
		tx.Create(&testItem{Name: "item1"})
		panic("boom")
	})
}

func TestWithTransaction_NestedDetection(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)

	err := tm.WithTransaction(context.Background(), func(outerCtx context.Context) error {
		outerTx := GetTxFromContext(outerCtx)
		if outerTx == nil {
			t.Fatal("expected transaction in outer context")
		}

		// Nested call — should reuse the same transaction, not begin a new one.
		return tm.WithTransaction(outerCtx, func(innerCtx context.Context) error {
			innerTx := GetTxFromContext(innerCtx)
			if innerTx == nil {
				t.Fatal("expected transaction in inner context")
			}
			// The inner tx should be the same pointer as the outer tx.
			if innerTx != outerTx {
				t.Error("nested transaction should reuse the outer transaction")
			}
			return innerTx.Create(&testItem{Name: "nested-item"}).Error
		})
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c := countItems(t, db); c != 1 {
		t.Errorf("expected 1 item after nested commit, got %d", c)
	}
}

func TestGetTxFromContext_NilWhenNoTransaction(t *testing.T) {
	tx := GetTxFromContext(context.Background())
	if tx != nil {
		t.Error("expected nil when no transaction in context")
	}
}

func TestGetDB_ReturnsTxWhenPresent(t *testing.T) {
	db := setupTestDB(t)
	tm := NewTransactionManager(db)

	_ = tm.WithTransaction(context.Background(), func(ctx context.Context) error {
		got := GetDB(ctx, db)
		tx := GetTxFromContext(ctx)
		// GetDB should return the transaction, not the base db.
		if got != tx {
			t.Error("GetDB should return the transaction from context")
		}
		return nil
	})
}

func TestGetDB_ReturnsFallbackDBWhenNoTx(t *testing.T) {
	db := setupTestDB(t)

	got := GetDB(context.Background(), db)
	if got == nil {
		t.Fatal("GetDB should return the fallback db")
	}
}
