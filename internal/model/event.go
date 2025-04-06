package model

import (
	"time"
)

type Event struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	ExpiredAt *time.Time
	EventType EventType `gorm:"not null"`
	Latitude  float64   `gorm:"not null"`
	Longitude float64   `gorm:"not null"`
	CreatedBy uint      `gorm:"not null"`
}
