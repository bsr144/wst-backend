package domain

import "github.com/shopspring/decimal"

type WasteCount struct {
	Type   PickupType
	Status PickupStatus
	Count  int
}

type WasteSummary struct {
	Counts []WasteCount
	Total  int
}

type PaymentStatusTotal struct {
	Status PaymentStatus
	Count  int
	Amount decimal.Decimal
}

type PaymentSummary struct {
	Totals       []PaymentStatusTotal
	TotalRevenue decimal.Decimal
}

type HouseholdHistory struct {
	Household Household
	Pickups   []Pickup
	Payments  []Payment
}
