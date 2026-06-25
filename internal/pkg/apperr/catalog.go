package apperr

const (
	CodeValidation         = "VALIDATION_ERROR"
	CodeInternal           = "INTERNAL_ERROR"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeRateLimited        = "RATE_LIMITED"
	CodeNotFound           = "NOT_FOUND"
	CodeMethodNotAllowed   = "METHOD_NOT_ALLOWED"
	CodeRequestError       = "REQUEST_ERROR"
)

var defaultMessages = map[string]string{
	CodeValidation:         "validation failed",
	CodeInternal:           "an unexpected error occurred, please try again later",
	CodeServiceUnavailable: "service temporarily unavailable",
	CodeRateLimited:        "too many requests",
	CodeNotFound:           "resource not found",
	CodeMethodNotAllowed:   "method not allowed",
	CodeRequestError:       "request could not be processed",
}

func MessageFor(code string) string {
	if msg, ok := defaultMessages[code]; ok {
		return msg
	}
	return defaultMessages[CodeInternal]
}
