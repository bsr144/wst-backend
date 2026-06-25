package dto

import "wst-backend/internal/core/domain"

type WasteCountResponse struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type WasteSummaryResponse struct {
	Counts []WasteCountResponse `json:"counts"`
	Total  int                  `json:"total"`
}

func NewWasteSummaryResponse(s domain.WasteSummary) WasteSummaryResponse {
	counts := make([]WasteCountResponse, 0, len(s.Counts))
	for _, c := range s.Counts {
		counts = append(counts, WasteCountResponse{
			Type:   string(c.Type),
			Status: string(c.Status),
			Count:  c.Count,
		})
	}
	return WasteSummaryResponse{Counts: counts, Total: s.Total}
}

type PaymentStatusTotalResponse struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
	Amount string `json:"amount"`
}

type PaymentSummaryResponse struct {
	Totals       []PaymentStatusTotalResponse `json:"totals"`
	TotalRevenue string                       `json:"total_revenue"`
}

func NewPaymentSummaryResponse(s domain.PaymentSummary) PaymentSummaryResponse {
	totals := make([]PaymentStatusTotalResponse, 0, len(s.Totals))
	for _, t := range s.Totals {
		totals = append(totals, PaymentStatusTotalResponse{
			Status: string(t.Status),
			Count:  t.Count,
			Amount: t.Amount.StringFixed(2),
		})
	}
	return PaymentSummaryResponse{Totals: totals, TotalRevenue: s.TotalRevenue.StringFixed(2)}
}

type HouseholdHistoryResponse struct {
	Household HouseholdResponse `json:"household"`
	Pickups   []PickupResponse  `json:"pickups"`
	Payments  []PaymentResponse `json:"payments"`
}

func NewHouseholdHistoryResponse(h domain.HouseholdHistory) HouseholdHistoryResponse {
	return HouseholdHistoryResponse{
		Household: NewHouseholdResponse(h.Household),
		Pickups:   NewPickupResponses(h.Pickups),
		Payments:  NewPaymentResponses(h.Payments),
	}
}
