package model

import "time"

type Vote struct {
	ID        uint      `gorm:"primarykey"`
	EventID   uint      `gorm:"not null"`
	UserID    string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	Upvote    bool      `gorm:"not null"`
}
