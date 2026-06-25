package dto

import (
	"time"

	"wst-backend/internal/core/domain"
)

type CreatePaymentRequest struct {
	HouseholdID string `json:"household_id" validate:"required"`
	WasteID     string `json:"waste_id" validate:"required"`
}

func (r *CreatePaymentRequest) Sanitize() {
	r.HouseholdID = sanitizeText(r.HouseholdID)
	r.WasteID = sanitizeText(r.WasteID)
}

type ListPaymentsQuery struct {
	Status      string `query:"status" json:"status" validate:"omitempty,oneof=pending paid failed"`
	HouseholdID string `query:"household_id" json:"household_id"`
	DateFrom    string `query:"date_from" json:"date_from"`
	DateTo      string `query:"date_to" json:"date_to"`
}

func (q *ListPaymentsQuery) Sanitize() {
	q.Status = sanitizeText(q.Status)
	q.HouseholdID = sanitizeText(q.HouseholdID)
	q.DateFrom = sanitizeText(q.DateFrom)
	q.DateTo = sanitizeText(q.DateTo)
}

type PaymentResponse struct {
	ID           string     `json:"id"`
	HouseholdID  string     `json:"household_id"`
	WasteID      string     `json:"waste_id"`
	Amount       string     `json:"amount"`
	PaymentDate  *time.Time `json:"payment_date"`
	Status       string     `json:"status"`
	ProofFileURL *string    `json:"proof_file_url"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func NewPaymentResponse(p domain.Payment) PaymentResponse {
	return PaymentResponse{
		ID:           p.ID.String(),
		HouseholdID:  p.HouseholdID.String(),
		WasteID:      p.WasteID.String(),
		Amount:       p.Amount.StringFixed(2),
		PaymentDate:  p.PaymentDate,
		Status:       string(p.Status),
		ProofFileURL: p.ProofFileURL,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func NewPaymentResponses(ps []domain.Payment) []PaymentResponse {
	out := make([]PaymentResponse, 0, len(ps))
	for _, p := range ps {
		out = append(out, NewPaymentResponse(p))
	}
	return out
}
