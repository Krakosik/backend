package repository

import (
	"errors"
	"fmt"
	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/model"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(user model.User) (model.User, error)
	GetByID(id string) (model.User, error)
	Save(user model.User) (model.User, error)
	DeleteByID(id string) error
}

type user struct {
	db *gorm.DB
}

func newUserRepository(db *gorm.DB) UserRepository {
	return &user{
		db: db,
	}
}

func (u *user) Create(user model.User) (model.User, error) {
	result := u.db.Create(&user)
	if result.Error != nil {
		return model.User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return user, nil
}

func (u *user) GetByID(id string) (model.User, error) {
	var user model.User
	result := u.db.First(&user, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.User{}, fmt.Errorf("%w: %v", dto.ErrNotFound, result.Error)
		}
		return model.User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}
	return user, nil
}

func (u *user) Save(user model.User) (model.User, error) {
	result := u.db.Save(&user)
	if result.Error != nil {
		return model.User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, result.Error)
	}

	return user, nil
}

func (u *user) DeleteByID(id string) error {
	result := u.db.Delete(&model.User{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("user cannot be deleted")
	}
	return nil
}
