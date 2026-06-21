package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PaymentStatus string

const (
	PaymentPending PaymentStatus = "pending"
	PaymentPaid    PaymentStatus = "paid"
	PaymentFailed  PaymentStatus = "failed"
)

type Payment struct {
	ID           uuid.UUID
	HouseholdID  uuid.UUID
	WasteID      uuid.UUID
	Amount       decimal.Decimal
	PaymentDate  *time.Time
	Status       PaymentStatus
	ProofFileURL *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
