package repository

import (
	"github.com/krakosik/backend/internal/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Repositories interface {
	User() UserRepository
	Event() EventRepository
}

type repositories struct {
	userRepository  UserRepository
	eventRepository EventRepository
}

func NewRepositories(db *gorm.DB) Repositories {
	err := db.AutoMigrate(&model.User{}, &model.Event{}, &model.Vote{})
	if err != nil {
		logrus.Panic(err)
	}
	userRepository := newUserRepository(db)
	eventRepository := newEventRepository(db)
	return &repositories{
		userRepository:  userRepository,
		eventRepository: eventRepository,
	}
}

func (r repositories) User() UserRepository {
	return r.userRepository
}

func (r repositories) Event() EventRepository {
	return r.eventRepository
}
