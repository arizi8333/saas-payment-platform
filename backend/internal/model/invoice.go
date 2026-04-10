package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InvoiceStatus string

const (
	InvoiceUnpaid InvoiceStatus = "unpaid"
	InvoicePaid   InvoiceStatus = "paid"
	InvoiceVoid   InvoiceStatus = "void"
)

type Invoice struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index"`
	TransactionID  *uuid.UUID     `gorm:"type:uuid;index"`
	SubscriptionID *uuid.UUID     `gorm:"type:uuid;index"`
	InvoiceNumber  string         `gorm:"type:varchar(50);not null;uniqueIndex"`
	Amount         int64          `gorm:"not null"`
	Currency       string         `gorm:"type:varchar(3);not null;default:'IDR'"`
	Status         InvoiceStatus  `gorm:"type:varchar(20);not null;default:'unpaid'"`
	DueDate        time.Time      `gorm:"not null"`
	PaidAt         *time.Time
	CreatedAt      time.Time      `gorm:"not null;default:now()"`
	UpdatedAt      time.Time      `gorm:"not null;default:now()"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	User         User          `gorm:"foreignKey:UserID"`
	Transaction  *Transaction  `gorm:"foreignKey:TransactionID"`
	Subscription *Subscription `gorm:"foreignKey:SubscriptionID"`
}
