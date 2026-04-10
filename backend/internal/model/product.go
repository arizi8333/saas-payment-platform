package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Product struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index"`
	Name        string         `gorm:"type:varchar(255);not null"`
	Description string         `gorm:"type:text"`
	IsActive    bool           `gorm:"not null;default:true"`
	CreatedAt   time.Time      `gorm:"not null;default:now()"`
	UpdatedAt   time.Time      `gorm:"not null;default:now()"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	User  User   `gorm:"foreignKey:UserID"`
	Plans []Plan `gorm:"foreignKey:ProductID"`
}
