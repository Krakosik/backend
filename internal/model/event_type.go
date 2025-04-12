package model

import "time"

type EventType int

const (
	EventTypePoliceCheck EventType = 0
	EventTypeAccident    EventType = 1
	EventTypeTrafficJam  EventType = 2
	EventTypeSpeedCamera EventType = 3
)

func (et EventType) GetExpirationDuration() time.Duration {
	switch et {
	case EventTypePoliceCheck, EventTypeAccident, EventTypeTrafficJam:
		return 30 * time.Minute
	case EventTypeSpeedCamera:
		return 30 * 24 * time.Hour
	default:
		return 30 * time.Minute
	}
}
