package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

type CtxKey string

var ErrMissingValue = errors.New("value is missing")

type TypeMismatchError struct {
	ExpectedType string
	ActualType   string
}

func (e *TypeMismatchError) Error() string {
	return fmt.Sprintf("type mismatch: expected %s, got %s", e.ExpectedType, e.ActualType)
}

func GetValueFromContext[T any](ctx context.Context, key CtxKey, dv T) (T, error) {
	v := ctx.Value(key)
	if v == nil {
		return dv, ErrMissingValue
	}
	val, ok := v.(T)
	if !ok {
		return dv, &TypeMismatchError{ExpectedType: fmt.Sprintf("%T", dv), ActualType: fmt.Sprintf("%T", v)}
	}
	return val, nil
}

type TokenHolder interface {
	CurrentToken() (string, error)
	IsExpired(time.Time) bool
	RefreshToken() error
	ResetToken()
}

func GetTokenValue(t TokenHolder) (string, error) {
	tt := slog.String("token_type", fmt.Sprintf("%T", t))

	slog.Debug("fetching the token value", tt)
	if t.IsExpired(time.Now().Add(time.Minute * 2)) {
		slog.Debug("refreshing expired token", tt)
		if err := t.RefreshToken(); err != nil {
			return "", err
		}
	}
	return t.CurrentToken()
}
