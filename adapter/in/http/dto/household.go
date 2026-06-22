package dto

import (
	"time"

	"wst-backend/core/domain"
)

type CreateHouseholdRequest struct {
	OwnerName string `json:"owner_name" validate:"required,max=255"`
	Address   string `json:"address" validate:"required,max=1000"`
}

type HouseholdResponse struct {
	ID        string    `json:"id"`
	OwnerName string    `json:"owner_name"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewHouseholdResponse(h domain.Household) HouseholdResponse {
	return HouseholdResponse{
		ID:        h.ID.String(),
		OwnerName: h.OwnerName,
		Address:   h.Address,
		CreatedAt: h.CreatedAt,
		UpdatedAt: h.UpdatedAt,
	}
}

func NewHouseholdResponses(hs []domain.Household) []HouseholdResponse {
	out := make([]HouseholdResponse, 0, len(hs))
	for _, h := range hs {
		out = append(out, NewHouseholdResponse(h))
	}
	return out
}
