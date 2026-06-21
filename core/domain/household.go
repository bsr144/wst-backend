package domain

import (
	"time"

	"github.com/google/uuid"
)

type Household struct {
	ID        uuid.UUID
	OwnerName string
	Address   string
	CreatedAt time.Time
	UpdatedAt time.Time
}
