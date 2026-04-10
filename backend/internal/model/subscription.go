package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BillingInterval string

const (
	BillingMonthly BillingInterval = "monthly"
	BillingYearly  BillingInterval = "yearly"
)

type Plan struct {
	ID              uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProductID       uuid.UUID       `gorm:"type:uuid;not null;index"`
	Name            string          `gorm:"type:varchar(255);not null"`
	Amount          int64           `gorm:"not null"`
	Currency        string          `gorm:"type:varchar(3);not null;default:'IDR'"`
	BillingInterval BillingInterval `gorm:"type:varchar(20);not null"`
	IsActive        bool            `gorm:"not null;default:true"`
	CreatedAt       time.Time       `gorm:"not null;default:now()"`
	UpdatedAt       time.Time       `gorm:"not null;default:now()"`
	DeletedAt       gorm.DeletedAt  `gorm:"index"`

	Product Product `gorm:"foreignKey:ProductID"`
}

type SubscriptionStatus string

const (
	SubStatusPendingPayment SubscriptionStatus = "pending_payment"
	SubStatusActive         SubscriptionStatus = "active"
	SubStatusCancelled      SubscriptionStatus = "cancelled"
	SubStatusExpired        SubscriptionStatus = "expired"
)

type Subscription struct {
	ID                 uuid.UUID          `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID             uuid.UUID          `gorm:"type:uuid;not null;index"`
	PlanID             uuid.UUID          `gorm:"type:uuid;not null;index"`
	Status             SubscriptionStatus `gorm:"type:varchar(30);not null;default:'pending_payment'"`
	CurrentPeriodStart time.Time          `gorm:"not null"`
	CurrentPeriodEnd   time.Time          `gorm:"not null"`
	CancelledAt        *time.Time
	CreatedAt          time.Time      `gorm:"not null;default:now()"`
	UpdatedAt          time.Time      `gorm:"not null;default:now()"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`

	User User `gorm:"foreignKey:UserID"`
	Plan Plan `gorm:"foreignKey:PlanID"`
}
