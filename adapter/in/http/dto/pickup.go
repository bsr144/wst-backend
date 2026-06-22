package dto

import (
	"time"

	"wst-backend/core/domain"
)

type CreatePickupRequest struct {
	HouseholdID string `json:"household_id" validate:"required"`
	Type        string `json:"type" validate:"required,oneof=organic plastic paper electronic"`
	SafetyCheck *bool  `json:"safety_check" validate:"required_if=Type electronic"`
}

type ListPickupsQuery struct {
	Status      string `query:"status" json:"status" validate:"omitempty,oneof=pending scheduled completed canceled"`
	HouseholdID string `query:"household_id" json:"household_id"`
}

type SchedulePickupRequest struct {
	PickupDate *time.Time `json:"pickup_date" validate:"required"`
}

type PickupResponse struct {
	ID          string     `json:"id"`
	HouseholdID string     `json:"household_id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	PickupDate  *time.Time `json:"pickup_date"`
	SafetyCheck bool       `json:"safety_check"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func NewPickupResponse(p domain.Pickup) PickupResponse {
	return PickupResponse{
		ID:          p.ID.String(),
		HouseholdID: p.HouseholdID.String(),
		Type:        string(p.Type),
		Status:      string(p.Status),
		PickupDate:  p.PickupDate,
		SafetyCheck: p.SafetyCheck,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func NewPickupResponses(ps []domain.Pickup) []PickupResponse {
	out := make([]PickupResponse, 0, len(ps))
	for _, p := range ps {
		out = append(out, NewPickupResponse(p))
	}
	return out
}
