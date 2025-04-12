package repository

import (
	"fmt"

	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/model"
	"gorm.io/gorm"
)

type EventRepository interface {
	Create(event model.Event) (model.Event, error)
	FindEventsWithinDistance(latitude, longitude float64, distanceInMeters float64) ([]model.Event, error)
	GetByID(id uint) (model.Event, error)
	CountVotes(eventID uint) (int32, error)
	Update(event model.Event) (model.Event, error)
}

type event struct {
	db *gorm.DB
}

func newEventRepository(db *gorm.DB) EventRepository {
	return &event{
		db: db,
	}
}

func (e *event) Create(event model.Event) (model.Event, error) {
	result := e.db.Create(&event)
	if result.Error != nil {
		return model.Event{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return event, nil
}

func (e *event) GetByID(id uint) (model.Event, error) {
	var event model.Event
	result := e.db.First(&event, id)
	if result.Error != nil {
		return model.Event{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return event, nil
}

func (e *event) FindEventsWithinDistance(latitude, longitude float64, distanceInMeters float64) ([]model.Event, error) {
	// Using Haversine formula directly in SQL with a subquery to allow HAVING on the calculated distance
	// The 6371000 is Earth's radius in meters
	haversineQuery := `
		SELECT e.* FROM (
			SELECT *, 
			(
				6371000 * acos(
					GREATEST(
						LEAST(
							cos(radians(?)) * 
							cos(radians(latitude)) * 
							cos(radians(longitude) - radians(?)) + 
							sin(radians(?)) * 
							sin(radians(latitude))
						, 1.0)
					, -1.0)
				)
			) AS distance 
			FROM events
			WHERE expired_at IS NULL OR expired_at > NOW()
		) AS e
		WHERE e.distance < ?
		ORDER BY e.distance
	`

	var events []model.Event
	result := e.db.Raw(
		haversineQuery,
		latitude,
		longitude,
		latitude,
		distanceInMeters,
	).Scan(&events)

	if result.Error != nil {
		return nil, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return events, nil
}

func (e *event) CountVotes(eventID uint) (int32, error) {
	var count int64
	result := e.db.Model(&model.Vote{}).Where("event_id = ? AND upvote = ?", eventID, true).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	// Count downvotes
	var downCount int64
	result = e.db.Model(&model.Vote{}).Where("event_id = ? AND upvote = ?", eventID, false).Count(&downCount)
	if result.Error != nil {
		return 0, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	// Calculate net votes
	netVotes := count - downCount
	return int32(netVotes), nil
}

func (e *event) Update(event model.Event) (model.Event, error) {
	result := e.db.Save(&event)
	if result.Error != nil {
		return model.Event{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return event, nil
}
