package household_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/internal/core/domain"
	repomock "wst-backend/internal/core/port/out/mock"
	"wst-backend/internal/core/service/household"
	"wst-backend/internal/core/service/servicetest"
)

func TestHouseholdService_Delete(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "ok", repoErr: nil, wantErr: nil},
		{name: "not found", repoErr: domain.ErrHouseholdNotFound, wantErr: domain.ErrHouseholdNotFound},
		{name: "dependents", repoErr: domain.ErrHouseholdHasDependents, wantErr: domain.ErrHouseholdHasDependents},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := new(repomock.HouseholdRepository)
			svc := household.NewService(repo, servicetest.FixedClock{At: time.Now()})
			repo.On("Delete", mock.Anything, id).Return(tc.repoErr).Once()

			err := svc.Delete(context.Background(), id)

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
			repo.AssertExpectations(t)
		})
	}
}
