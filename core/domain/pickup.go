package domain

import (
	"time"

	"github.com/google/uuid"
)

type PickupType string

const (
	PickupOrganic    PickupType = "organic"
	PickupPlastic    PickupType = "plastic"
	PickupPaper      PickupType = "paper"
	PickupElectronic PickupType = "electronic"
)

func (t PickupType) Valid() bool {
	switch t {
	case PickupOrganic, PickupPlastic, PickupPaper, PickupElectronic:
		return true
	default:
		return false
	}
}

type PickupStatus string

const (
	PickupPending   PickupStatus = "pending"
	PickupScheduled PickupStatus = "scheduled"
	PickupCompleted PickupStatus = "completed"
	PickupCanceled  PickupStatus = "canceled"
)

type Pickup struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	Type        PickupType
	Status      PickupStatus
	PickupDate  *time.Time
	SafetyCheck bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p Pickup) CanSchedule() error {
	if p.Status != PickupPending {
		return ErrPickupNotPending
	}
	if p.Type == PickupElectronic && !p.SafetyCheck {
		return ErrSafetyCheckRequired
	}
	return nil
}
