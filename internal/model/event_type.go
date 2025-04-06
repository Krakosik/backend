package model

type EventType int

const (
	EventTypePoliceCheck EventType = 0
	EventTypeAccident    EventType = 1
	EventTypeTrafficJam  EventType = 2
	EventTypeSpeedCamera EventType = 3
)
