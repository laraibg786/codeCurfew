package common

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type ctxKey string

const (
	loggerKey = ctxKey("logger")
	jwtKey    = ctxKey("jwt_token")
)

func GetLoggerFromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(loggerKey).(*slog.Logger)
	if !ok {
		l = slog.Default()
		l.Warn("logger not found in request context. using default logger")
	}
	return l
}

func GetJWTFromContext(ctx context.Context, l *slog.Logger) (TokenHolder, error) {
	t, ok := ctx.Value(jwtKey).(TokenHolder)
	if !ok {
		l.Warn("jwt token not found in context")
		return nil, fmt.Errorf("jwt token not found in context")
	}

func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func ContextWithJWTToken(ctx context.Context, token TokenHolder) context.Context {
	return context.WithValue(ctx, jwtKey, token)
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
