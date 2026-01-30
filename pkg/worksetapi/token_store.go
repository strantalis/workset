package worksetapi

import (
	"context"
	"errors"
)

// ErrTokenNotFound indicates no token was stored.
var ErrTokenNotFound = errors.New("auth token not found")

// TokenStore persists authentication tokens.
type TokenStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

const (
	tokenStoreKey    = "github.com"
	tokenSourceKey   = "github.com.source"
	tokenAuthModeKey = "github.com.auth_mode"
)
