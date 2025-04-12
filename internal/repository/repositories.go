package repository

import (
	"github.com/krakosik/backend/internal/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Repositories interface {
	User() UserRepository
	Event() EventRepository
	Vote() VoteRepository
}

type repositories struct {
	userRepository  UserRepository
	eventRepository EventRepository
	voteRepository  VoteRepository
}

func NewRepositories(db *gorm.DB) Repositories {
	err := db.AutoMigrate(&model.User{}, &model.Event{}, &model.Vote{})
	if err != nil {
		logrus.Panic(err)
	}
	userRepository := newUserRepository(db)
	eventRepository := newEventRepository(db)
	voteRepository := newVoteRepository(db)
	return &repositories{
		userRepository:  userRepository,
		eventRepository: eventRepository,
		voteRepository:  voteRepository,
	}
}

func (r repositories) User() UserRepository {
	return r.userRepository
}

func (r repositories) Event() EventRepository {
	return r.eventRepository
}

func (r repositories) Vote() VoteRepository {
	return r.voteRepository
}
