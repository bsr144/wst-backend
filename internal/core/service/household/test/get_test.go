package household_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/household"
	"wst-backend/internal/core/service/servicetest"
)

func TestHouseholdService_Get(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	want := domain.Household{ID: id, OwnerName: "Budi", Address: "Jl. Mawar 1", CreatedAt: now, UpdatedAt: now}

	tests := []struct {
		name    string
		setup   func(*repomock.HouseholdRepository)
		wantErr error
		want    domain.Household
	}{
		{
			name: "found",
			setup: func(r *repomock.HouseholdRepository) {
				r.On("FindByID", mock.Anything, id).Return(want, nil).Once()
			},
			want: want,
		},
		{
			name: "not found",
			setup: func(r *repomock.HouseholdRepository) {
				r.On("FindByID", mock.Anything, id).Return(domain.Household{}, domain.ErrHouseholdNotFound).Once()
			},
			wantErr: domain.ErrHouseholdNotFound,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(repomock.HouseholdRepository)
			svc := household.NewService(repo, servicetest.FixedClock{At: now})
			tc.setup(repo)

			got, err := svc.Get(context.Background(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
			repo.AssertExpectations(t)
		})
	}
}
