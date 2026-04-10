package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WebhookEndpoint struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID      `gorm:"type:uuid;not null;index"`
	URL       string         `gorm:"type:varchar(500);not null"`
	Secret    string         `gorm:"type:varchar(255);not null"`
	Events    datatypes.JSON `gorm:"type:jsonb;not null"`
	IsActive  bool           `gorm:"not null;default:true"`
	CreatedAt time.Time      `gorm:"not null;default:now()"`
	UpdatedAt time.Time      `gorm:"not null;default:now()"`
	DeletedAt gorm.DeletedAt `gorm:"index"`

	User User `gorm:"foreignKey:UserID"`
}

type DeliveryStatus string

const (
	DeliveryPending   DeliveryStatus = "pending"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryFailed    DeliveryStatus = "failed"
)

type WebhookDelivery struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	WebhookEndpointID uuid.UUID      `gorm:"type:uuid;not null;index"`
	EventType         string         `gorm:"type:varchar(50);not null"`
	Payload           datatypes.JSON `gorm:"type:jsonb;not null"`
	Status            DeliveryStatus `gorm:"type:varchar(20);not null;default:'pending'"`
	ResponseCode      *int
	ResponseBody      *string    `gorm:"type:text"`
	RetryCount        int        `gorm:"not null;default:0"`
	MaxRetries        int        `gorm:"not null;default:5"`
	NextRetryAt       *time.Time `gorm:"index"`
	DeliveredAt       *time.Time
	CreatedAt         time.Time `gorm:"not null;default:now()"`
	UpdatedAt         time.Time `gorm:"not null;default:now()"`

	WebhookEndpoint WebhookEndpoint `gorm:"foreignKey:WebhookEndpointID"`
}
