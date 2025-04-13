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

type User = model.User

type AuthService interface {
	ValidateToken(ctx context.Context, token string) (User, error)
}

type authService struct {
	userRepository      repository.UserRepository
	authClient          client.AuthClient
	tokenExpireVerifier client.TokenExpireVerifier
}

func newAuthService(userRepository repository.UserRepository, authClient client.AuthClient, verifier client.TokenExpireVerifier) AuthService {
	return &authService{userRepository: userRepository, authClient: authClient, tokenExpireVerifier: verifier}
}

func (a *authService) ValidateToken(ctx context.Context, token string) (User, error) {
	var newUser User

	response, err := a.authClient.VerifyIDToken(ctx, token)
	if err != nil {
		if a.tokenExpireVerifier(err) {
			return User{}, fmt.Errorf("%w: %v", dto.ErrNotAuthorized, err)
		}
		return User{}, fmt.Errorf("%w: %v", dto.ErrInternalFailure, err)
	}

	var userEmail string
	if email, ok := response.Claims["email"]; ok {
		if emailStr, ok := email.(string); ok {
			userEmail = emailStr
		}
	}

	user, err := a.userRepository.GetByID(response.UID)

	if err != nil {
		if errors.Is(err, dto.ErrNotFound) {
			newUser, err = a.userRepository.Create(User{
				ID:    response.UID,
				Email: userEmail,
			})
			if err != nil {
				return User{}, err // internal error
			}
			return newUser, nil
		}
		return User{}, err
	}

	if user.Email != userEmail {
		user.Email = userEmail

		_, err = a.userRepository.Save(user)
		if err != nil {
			return User{}, err
		}
	}

	return user, nil
}
