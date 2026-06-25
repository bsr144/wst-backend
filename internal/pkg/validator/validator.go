package validator

import (
	"reflect"
	"strings"
	"sync"

	playground "github.com/go-playground/validator/v10"

	"wst-backend/internal/pkg/apperr"
)

var (
	once     sync.Once
	validate *playground.Validate
)

func instance() *playground.Validate {
	once.Do(func() {
		validate = playground.New(playground.WithRequiredStructEnabled())
		validate.RegisterTagNameFunc(func(field reflect.StructField) string {
			name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	})
	return validate
}

func Struct(s any) *apperr.Error {
	err := instance().Struct(s)
	if err == nil {
		return nil
	}
	var verrs playground.ValidationErrors
	if !asValidationErrors(err, &verrs) {
		return apperr.Validation(apperr.CodeValidation, "invalid request")
	}
	details := make([]apperr.FieldError, 0, len(verrs))
	for _, fe := range verrs {
		details = append(details, apperr.FieldError{
			Field:  fe.Field(),
			Reason: reason(fe),
		})
	}
	return apperr.Validation(apperr.CodeValidation, "validation failed").WithDetails(details...)
}

func asValidationErrors(err error, target *playground.ValidationErrors) bool {
	if verrs, ok := err.(playground.ValidationErrors); ok {
		*target = verrs
		return true
	}
	return false
}

func reason(fe playground.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "required_if", "required_unless", "required_with", "required_without":
		if parts := strings.Fields(fe.Param()); len(parts) == 2 {
			return "is required when " + strings.ToLower(parts[0]) + " is " + parts[1]
		}
		return "is required"
	case "oneof":
		return "must be one of: " + fe.Param()
	case "uuid", "uuid4":
		return "must be a valid uuid"
	case "min":
		return "must be at least " + fe.Param()
	case "max":
		return "must be at most " + fe.Param()
	case "email":
		return "must be a valid email"
	case "e164":
		return "must be a valid phone number"
	case "gt":
		return "must be greater than " + fe.Param()
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	default:
		return "is invalid"
	}
}
