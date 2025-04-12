package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/krakosik/backend/internal/client"
	"github.com/krakosik/backend/internal/dto"
	"github.com/krakosik/backend/internal/model"
	"github.com/krakosik/backend/internal/repository"
)

type AuthService interface {
	ValidateToken(ctx context.Context, token string) (model.User, error)
}

type authService struct {
	userRepository      repository.UserRepository
	authClient          client.AuthClient
	tokenExpireVerifier client.TokenExpireVerifier
}

func newAuthService(userRepository repository.UserRepository, authClient client.AuthClient, verifier client.TokenExpireVerifier) AuthService {
	return &authService{userRepository: userRepository, authClient: authClient, tokenExpireVerifier: verifier}
}

func (a *authService) ValidateToken(ctx context.Context, token string) (model.User, error) {
	var newUser model.User

	response, err := a.authClient.VerifyIDToken(ctx, token)
	if err != nil {
		if a.tokenExpireVerifier(err) {
			return model.User{}, fmt.Errorf("%w: %v", dto.ErrNotAuthorized, err)
		}
		return model.User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, err)
	}

	if _, ok := response.Claims["email"]; !ok {
		return model.User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, "email claim not found")

	}
	if _, ok := response.Claims["email"].(string); !ok {
		return model.User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, "email claim is not a string")
	}

	userEmail := response.Claims["email"].(string)

	user, err := a.userRepository.GetByID(response.UID)

	if err != nil {
		if errors.Is(err, dto.ErrNotFound) {
			newUser, err = a.userRepository.Create(model.User{
				ID:    response.UID,
				Email: userEmail,
			})
			if err != nil {
				return model.User{}, err // internal error
			}
			return newUser, nil
		}
		return model.User{}, err
	}

	if user.Email != userEmail {
		user.Email = userEmail

		_, err = a.userRepository.Save(user)
		if err != nil {
			return model.User{}, err
		}
	}

	return user, nil
}
