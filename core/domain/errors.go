package domain

import "wst-backend/pkg/apperr"

var (
	ErrHouseholdNotFound          = apperr.NotFound("HOUSEHOLD_NOT_FOUND", "household not found")
	ErrHouseholdHasPendingPayment = apperr.Conflict("HOUSEHOLD_HAS_PENDING_PAYMENT", "household has a pending payment")
	ErrHouseholdHasDependents     = apperr.Conflict("HOUSEHOLD_HAS_DEPENDENTS", "household has pickups or payments and cannot be deleted")

	ErrPickupNotFound      = apperr.NotFound("PICKUP_NOT_FOUND", "pickup not found")
	ErrPickupNotPending    = apperr.Conflict("PICKUP_NOT_PENDING", "pickup must be pending for this action")
	ErrPickupNotScheduled  = apperr.Conflict("PICKUP_NOT_SCHEDULED", "pickup must be scheduled to complete")
	ErrSafetyCheckRequired = apperr.Unprocessable("SAFETY_CHECK_REQUIRED", "electronic pickup requires a passed safety check")
	ErrPickupNotCancelable = apperr.Conflict("PICKUP_NOT_CANCELABLE", "pickup cannot be canceled in its current status")

	ErrPaymentNotFound   = apperr.NotFound("PAYMENT_NOT_FOUND", "payment not found")
	ErrPaymentNotPending = apperr.Conflict("PAYMENT_NOT_PENDING", "payment must be pending to confirm")
)
