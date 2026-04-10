package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type APIKey struct {
	ID         uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	Name       string         `gorm:"type:varchar(100);not null"`
	KeyHash    string         `gorm:"type:varchar(255);not null;uniqueIndex"`
	KeyPrefix  string         `gorm:"type:varchar(12);not null"`
	IsActive   bool           `gorm:"not null;default:true"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	RateLimit  int            `gorm:"not null;default:1000"`
	CreatedAt  time.Time      `gorm:"not null;default:now()"`
	UpdatedAt  time.Time      `gorm:"not null;default:now()"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`

	User User `gorm:"foreignKey:UserID"`
}
