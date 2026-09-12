package auth

import (
	"context"

	database "edugrade-enterprise/services/api-gateway/internal/db"
)

type contextKey string

const userContextKey contextKey = "auth_user"

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(database.WithTenant(ctx, user.TenantID), userContextKey, user)
}

func UserFromContext(ctx context.Context) (User, bool) {
	user, ok := ctx.Value(userContextKey).(User)
	return user, ok
}
