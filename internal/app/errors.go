package app

import (
	"context"
	"errors"
)

// UserError 是返回给前端的稳定错误形态（code + 可读 message），避免直接抛内部细节。
type UserError struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	DiagnosticRef string `json:"diagnosticRef,omitempty"`
}

func ToUserError(err error) UserError {
	if err == nil {
		return UserError{}
	}
	if errors.Is(err, context.Canceled) {
		return UserError{Code: "cancelled", Message: "Operation cancelled."}
	}
	return UserError{Code: "operation_failed", Message: err.Error()}
}
