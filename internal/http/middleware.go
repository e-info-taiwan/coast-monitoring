package httpx

import (
	"context"
	"net/http"

	"coast-monitoring/internal/policy"
)

type contextKey string

const currentUserKey contextKey = "currentUser"

func withCurrentUser(ctx context.Context, user policy.User) context.Context {
	return context.WithValue(ctx, currentUserKey, user)
}

func currentUser(r *http.Request) (policy.User, bool) {
	user, ok := r.Context().Value(currentUserKey).(policy.User)
	return user, ok
}
