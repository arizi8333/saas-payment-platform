package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string

const (
	RoleAdmin     UserRole = "admin"
	RoleDeveloper UserRole = "developer"
	RoleEndUser   UserRole = "end_user"
)

type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Email        string         `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash string         `gorm:"type:varchar(255);not null"`
	FullName     string         `gorm:"type:varchar(255);not null"`
	Role         UserRole       `gorm:"type:varchar(20);not null;default:'developer'"`
	IsActive     bool           `gorm:"not null;default:true"`
	CreatedAt    time.Time      `gorm:"not null;default:now()"`
	UpdatedAt    time.Time      `gorm:"not null;default:now()"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}
