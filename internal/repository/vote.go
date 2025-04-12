package repository

import (
	"fmt"

	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/model"
	"gorm.io/gorm"
)

type VoteRepository interface {
	Create(vote model.Vote) (model.Vote, error)
	CountVotes(eventID uint) (int32, error)
}

type vote struct {
	db *gorm.DB
}

func newVoteRepository(db *gorm.DB) VoteRepository {
	return &vote{
		db: db,
	}
}

func (v *vote) Create(vote model.Vote) (model.Vote, error) {
	result := v.db.Create(&vote)
	if result.Error != nil {
		return model.Vote{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return vote, nil
}

func (v *vote) CountVotes(eventID uint) (int32, error) {
	var upvotes int64
	result := v.db.Model(&model.Vote{}).Where("event_id = ? AND upvote = ?", eventID, true).Count(&upvotes)
	if result.Error != nil {
		return 0, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	var downvotes int64
	result = v.db.Model(&model.Vote{}).Where("event_id = ? AND upvote = ?", eventID, false).Count(&downvotes)
	if result.Error != nil {
		return 0, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return int32(upvotes - downvotes), nil
}
