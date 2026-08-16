package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

type ErrorBody struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Problem struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
}

func (p *Problem) Error() string { return p.Message }

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, err error) {
	var p *Problem
	if errors.As(err, &p) {
		WriteJSON(w, p.Status, ErrorBody{Error: APIError{Code: p.Code, Message: p.Message, Details: p.Details}})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, ErrorBody{Error: APIError{Code: "internal_error", Message: "unexpected server error"}})
}

func BadRequest(code, msg string) *Problem {
	return &Problem{Status: http.StatusBadRequest, Code: code, Message: msg}
}
func Unauthorized(code, msg string) *Problem {
	return &Problem{Status: http.StatusUnauthorized, Code: code, Message: msg}
}
func Forbidden(code, msg string) *Problem {
	return &Problem{Status: http.StatusForbidden, Code: code, Message: msg}
}
func Conflict(code, msg string) *Problem {
	return &Problem{Status: http.StatusConflict, Code: code, Message: msg}
}
func NotFound(code, msg string) *Problem {
	return &Problem{Status: http.StatusNotFound, Code: code, Message: msg}
}
