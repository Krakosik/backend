package service

import (
	"sync"
)

type LocationInfo struct {
	Latitude  float64
	Longitude float64
	Timestamp int64
}

type LocationService interface {
	UpdateLocation(id string, latitude, longitude float64, timestamp int64)
	GetLocation(id string) (LocationInfo, bool)
	RemoveLocation(id string)
}

type locationService struct {
	locationCache map[string]LocationInfo
	locationMutex sync.RWMutex
}

func newLocationService() LocationService {
	return &locationService{
		locationCache: make(map[string]LocationInfo),
	}
}

func (s *locationService) UpdateLocation(id string, latitude, longitude float64, timestamp int64) {
	s.locationMutex.Lock()
	defer s.locationMutex.Unlock()

	s.locationCache[id] = LocationInfo{
		Latitude:  latitude,
		Longitude: longitude,
		Timestamp: timestamp,
	}
}

func (s *locationService) GetLocation(id string) (LocationInfo, bool) {
	s.locationMutex.RLock()
	defer s.locationMutex.RUnlock()

	location, exists := s.locationCache[id]
	return location, exists
}

func (s *locationService) RemoveLocation(id string) {
	s.locationMutex.Lock()
	defer s.locationMutex.Unlock()

	delete(s.locationCache, id)
}
