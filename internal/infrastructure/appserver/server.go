// Package appserver is the HTTP layer: routing, error mapping, request logging and correlation ids.
package appserver

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/logger"
)

// CORRELATION_HEADER is propagated from the caller (or generated) and echoed on the response.
const CORRELATION_HEADER = "X-Correlation-ID"

// HandlerFunc is a controller method: it writes the success response itself and returns an error otherwise.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// handle maps a controller's error to a response: httperror codes as-is, anything else a logged 500
// whose details stay in the log, not the response.
func handle(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		var be httperror.BSError
		if errors.As(err, &be) {
			http.Error(w, be.Error(), be.GetErrorCode())
			return
		}
		logger.FromContext(r.Context()).Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		http.Error(w, "internal error; see server log with correlation id "+w.Header().Get(CORRELATION_HEADER), httperror.INTERNAL_ERROR)
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// withRequestContext attaches a correlation id and request-scoped logger, and logs each request.
func withRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(CORRELATION_HEADER)
		if id == "" {
			b := make([]byte, 8)
			rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set(CORRELATION_HEADER, id)
		ctx := logger.WithCorrelationID(r.Context(), id)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))
		// Only the path is logged: query strings may carry OAuth codes.
		logger.FromContext(ctx).Info("http request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}
