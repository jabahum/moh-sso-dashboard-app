package modulekit

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Meta, ErrorBody and the envelope types reproduce the host's response schema.
type Meta struct {
	RequestID string `json:"requestId,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type successEnvelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Meta    Meta `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
	Meta    Meta      `json:"meta,omitempty"`
}

// Error is an HTTP failure with the platform's error code and message.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string   { return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message) }
func (e *Error) StatusCode() int { return e.Status }

// NewError builds an HTTP failure.
func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// MetaFor returns the envelope metadata for a request.
func MetaFor(r *http.Request) Meta {
	request, _ := FromRequest(r)
	return Meta{RequestID: request.RequestID}
}

// WriteJSON writes any JSON value, encoded like the host (HTML-escaped, no
// trailing newline).
func WriteJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteOK writes a success envelope.
func WriteOK(w http.ResponseWriter, r *http.Request, status int, data any) {
	WriteJSON(w, status, successEnvelope{Success: true, Data: data, Meta: MetaFor(r)})
}

// WriteFail writes an error envelope.
func WriteFail(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteJSON(w, status, errorEnvelope{Success: false, Error: ErrorBody{Code: code, Message: message}, Meta: MetaFor(r)})
}

// WriteError writes err as an error envelope. *Error values keep their status,
// code and message; anything else becomes a generic 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if failure, ok := err.(*Error); ok && failure != nil {
		WriteFail(w, r, failure.Status, failure.Code, failure.Message)
		return
	}
	WriteFail(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
}
