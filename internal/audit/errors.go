package audit

import (
	"errors"
	"fmt"
)

var (
	// ErrAuditWriteFailed is returned when writing to the audit log fails.
	ErrAuditWriteFailed = errors.New("audit write failed")
	// ErrAuditEventInvalid is returned when the audit event is malformed.
	ErrAuditEventInvalid = errors.New("invalid audit event")
)

// AuditError represents an audit-specific error.
type AuditError struct {
	Err    error
	Code   string
	Detail map[string]string
}

// Error returns the error message.
func (e *AuditError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

// Unwrap returns the underlying error.
func (e *AuditError) Unwrap() error {
	return e.Err
}

// NewAuditError creates a new AuditError.
func NewAuditError(err error, code string, detail map[string]string) *AuditError {
	return &AuditError{Err: err, Code: code, Detail: detail}
}
