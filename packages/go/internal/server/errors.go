package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/b4moss/es4/packages/go/pkg/es4"
)

// Error codes returned in HTTP error bodies.
const (
	CodeNotFound      = "not_found"
	CodeInvalidKey    = "invalid_key"
	CodeInvalidValue  = "invalid_value"
	CodeConflict      = "conflict"
	CodeNotReady      = "not_ready"
	CodeInternal      = "internal"
)

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{Error: errorDetail{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func mapStateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, es4.ErrNotFound):
		writeError(w, http.StatusNotFound, CodeNotFound, err.Error())
	case errors.Is(err, es4.ErrInvalidKey):
		writeError(w, http.StatusBadRequest, CodeInvalidKey, err.Error())
	case errors.Is(err, es4.ErrInvalidValue):
		writeError(w, http.StatusBadRequest, CodeInvalidValue, err.Error())
	case errors.Is(err, es4.ErrNestedTx), errors.Is(err, es4.ErrTxDone):
		writeError(w, http.StatusConflict, CodeConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, CodeInternal, err.Error())
	}
}
