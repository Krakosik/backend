package model

import "time"

type Vote struct {
	ID        uint `gorm:"primarykey"`
	EventID   uint `gorm:"not null"`
	CreatedAt time.Time
	Upvote    bool `gorm:"not null"`
}
