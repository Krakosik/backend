package ctx

import (
	"context"

	"github.com/krakosik/backend/internal/model"
)

type contextKey string

const (
	UserContextKey contextKey = "user"
)

type User = model.User

func GetUserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(UserContextKey).(User)
	return user, ok
}
