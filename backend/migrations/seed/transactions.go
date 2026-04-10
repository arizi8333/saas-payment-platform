package seed

import (
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/saas-payment-platform/backend/internal/model"
)

var (
	seedTxID1 = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	seedTxID2 = uuid.MustParse("00000000-0000-0000-0000-000000000021")
	seedTxID3 = uuid.MustParse("00000000-0000-0000-0000-000000000022")
)

// seedTransactions creates sample transactions for the developer user.
// Uses FirstOrCreate keyed on external_id for idempotency.
func seedTransactions(db *gorm.DB, devUserID uuid.UUID) error {
	now := time.Now()

	txns := []model.Transaction{
		{
			ID:            seedTxID1,
			UserID:        devUserID,
			ExternalID:    "seed-tx-001",
			Amount:        150000,
			Currency:      "IDR",
			Status:        model.TxStatusSuccess,
			PaymentMethod: model.PMBankTransfer,
			Description:   "Sample successful bank transfer",
			CustomerEmail: "customer1@dev.local",
			Metadata:      datatypes.JSON([]byte(`{"source":"seed"}`)),
			PaidAt:        &now,
		},
		{
			ID:            seedTxID2,
			UserID:        devUserID,
			ExternalID:    "seed-tx-002",
			Amount:        75000,
			Currency:      "IDR",
			Status:        model.TxStatusPending,
			PaymentMethod: model.PMEWallet,
			Description:   "Sample pending e-wallet payment",
			CustomerEmail: "customer2@dev.local",
			Metadata:      datatypes.JSON([]byte(`{"source":"seed"}`)),
		},
		{
			ID:            seedTxID3,
			UserID:        devUserID,
			ExternalID:    "seed-tx-003",
			Amount:        250000,
			Currency:      "IDR",
			Status:        model.TxStatusFailed,
			PaymentMethod: model.PMCreditCard,
			Description:   "Sample failed credit card payment",
			CustomerEmail: "customer3@dev.local",
			Metadata:      datatypes.JSON([]byte(`{"source":"seed"}`)),
		},
	}

	for _, tx := range txns {
		if err := db.Where("external_id = ?", tx.ExternalID).FirstOrCreate(&tx).Error; err != nil {
			return err
		}
		slog.Info("seed transaction", "external_id", tx.ExternalID, "status", tx.Status, "amount", tx.Amount)
	}

	return nil
}
