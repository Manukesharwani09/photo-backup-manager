// Package httperror is the error type services return so controllers can map failures to HTTP
// status codes. Anything that is not an HTTPError becomes a 500 with a generic message.
package httperror

import "fmt"

const (
	BAD_REQUEST    = 400
	NOT_FOUND      = 404
	CONFLICT       = 409
	INTERNAL_ERROR = 500
	BAD_GATEWAY    = 502
)

// BSError is an error that knows its HTTP status code.
type BSError interface {
	error
	GetErrorCode() int
}

type HTTPError struct {
	Code    int
	Message string
}

func (e *HTTPError) Error() string     { return e.Message }
func (e *HTTPError) GetErrorCode() int { return e.Code }

func NewHTTPError(code int, message string) BSError {
	return &HTTPError{Code: code, Message: message}
}

func NewHTTPErrorf(code int, format string, args ...any) BSError {
	return &HTTPError{Code: code, Message: fmt.Sprintf(format, args...)}
}
