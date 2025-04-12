package model

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID        string `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	FirstName *string
	LastName  *string
	Email     string `gorm:"required; not null"`
}
