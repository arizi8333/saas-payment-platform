package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TransactionStatus string

const (
	TxStatusPending TransactionStatus = "pending"
	TxStatusSuccess TransactionStatus = "success"
	TxStatusFailed  TransactionStatus = "failed"
	TxStatusExpired TransactionStatus = "expired"
)

type PaymentMethod string

const (
	PMBankTransfer PaymentMethod = "bank_transfer"
	PMCreditCard   PaymentMethod = "credit_card"
	PMEWallet      PaymentMethod = "e_wallet"
	PMQRCode       PaymentMethod = "qr_code"
)

type Transaction struct {
	ID             uuid.UUID         `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID         uuid.UUID         `gorm:"type:uuid;not null;index"`
	ExternalID     string            `gorm:"type:varchar(255);not null;uniqueIndex"`
	Amount         int64             `gorm:"not null"`
	Currency       string            `gorm:"type:varchar(3);not null;default:'IDR'"`
	Status         TransactionStatus `gorm:"type:varchar(20);not null;default:'pending'"`
	PaymentMethod  PaymentMethod     `gorm:"type:varchar(30);not null"`
	Description    string            `gorm:"type:text"`
	CustomerEmail  string            `gorm:"type:varchar(255)"`
	IdempotencyKey *string           `gorm:"type:varchar(255);uniqueIndex"`
	Metadata       datatypes.JSON    `gorm:"type:jsonb"`
	PaidAt         *time.Time
	ExpiredAt      *time.Time
	CreatedAt      time.Time         `gorm:"not null;default:now()"`
	UpdatedAt      time.Time         `gorm:"not null;default:now()"`
	DeletedAt      gorm.DeletedAt    `gorm:"index"`

	User User `gorm:"foreignKey:UserID"`
}
