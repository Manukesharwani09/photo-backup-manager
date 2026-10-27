// Package logger is the structured logger (log/slog) plus PII masking helpers.
// Never log raw emails or tokens: use MaskEmail.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

type ctxKey struct{}

var base = slog.New(slog.NewTextHandler(os.Stderr, nil))

// GetLogger returns the process logger.
func GetLogger() *slog.Logger { return base }

// WithCorrelationID returns a context whose logger tags every line with the request's correlation id.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, base.With("correlation_id", id))
}

// FromContext returns the request logger (with correlation id) or the process logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return base
}

// MaskEmail turns "someone@gmail.com" into "som***@gmail.com".
func MaskEmail(email string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok {
		return "***"
	}
	if len(local) > 3 {
		local = local[:3]
	}
	return local + "***@" + domain
}
