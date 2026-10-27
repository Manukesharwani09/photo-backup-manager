// Package common holds small helpers shared by controllers.
package common

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Manukesharwani09/photo-backup-manager/internal/infrastructure/httperror"
)

// MaxBodyBytes caps request bodies (a sync of ~100k files fits comfortably).
const MaxBodyBytes = 32 << 20

// DecodeJSON reads a JSON body into v, rejecting unknown fields so app/backend contract drift is caught.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return httperror.NewHTTPError(httperror.BAD_REQUEST, "invalid JSON: "+err.Error())
	}
	return nil
}

// WriteJSON writes v as indented JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// FormatGB renders bytes as "12.08 GB"; negative means unlimited.
func FormatGB(b int64) string {
	if b < 0 {
		return "unlimited"
	}
	return strconv.FormatFloat(float64(int(float64(b)/(1<<30)*100))/100, 'f', -1, 64) + " GB"
}

// PathInt64 parses a numeric path parameter.
func PathInt64(r *http.Request, name string) (int64, error) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, httperror.NewHTTPErrorf(httperror.BAD_REQUEST, "bad %s", name)
	}
	return v, nil
}
