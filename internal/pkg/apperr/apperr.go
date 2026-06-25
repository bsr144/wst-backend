package apperr

import "errors"

type Kind int

const (
	KindInternal Kind = iota
	KindValidation
	KindNotFound
	KindConflict
	KindUnprocessable
	KindUnavailable
	KindRateLimited
)

func (k Kind) Status() int {
	switch k {
	case KindValidation:
		return 400
	case KindNotFound:
		return 404
	case KindConflict:
		return 409
	case KindUnprocessable:
		return 422
	case KindRateLimited:
		return 429
	case KindUnavailable:
		return 503
	default:
		return 500
	}
}

func (k Kind) IsServerError() bool { return k.Status() >= 500 }

type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Details []FieldError
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.cause }

func (e *Error) Cause() error { return e.cause }

func (e *Error) Status() int { return e.Kind.Status() }

func (e *Error) IsServerError() bool { return e.Kind.IsServerError() }

func (e *Error) ClientCode() string {
	if e.Kind == KindInternal {
		return CodeInternal
	}
	return e.Code
}

func (e *Error) ClientMessage() string {
	if e.Kind == KindInternal {
		return MessageFor(CodeInternal)
	}
	return e.Message
}

func (e *Error) WithCause(err error) *Error {
	clone := *e
	clone.cause = err
	return &clone
}

func (e *Error) WithDetails(details ...FieldError) *Error {
	clone := *e
	clone.Details = details
	return &clone
}

func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

func Validation(code, message string) *Error    { return New(KindValidation, code, message) }
func NotFound(code, message string) *Error      { return New(KindNotFound, code, message) }
func Conflict(code, message string) *Error      { return New(KindConflict, code, message) }
func Unprocessable(code, message string) *Error { return New(KindUnprocessable, code, message) }
func Unavailable(code, message string) *Error   { return New(KindUnavailable, code, message) }
func RateLimited(code, message string) *Error   { return New(KindRateLimited, code, message) }
func Internal(code, message string) *Error      { return New(KindInternal, code, message) }

func From(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
