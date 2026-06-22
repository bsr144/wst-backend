package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"wst-backend/core/domain"
	"wst-backend/core/port/in"
	repomock "wst-backend/core/port/out/mock"
	"wst-backend/core/service"
	"wst-backend/pkg/pagination"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

func TestHouseholdService_Create(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	repo := new(repomock.HouseholdRepository)
	svc := service.NewHouseholdService(repo, fixedClock{now: now})

	repo.On("Insert", mock.Anything, mock.MatchedBy(func(h domain.Household) bool {
		return h.OwnerName == "Budi" &&
			h.Address == "Jl. Mawar 1" &&
			h.CreatedAt.Equal(now) &&
			h.UpdatedAt.Equal(now) &&
			h.ID != uuid.Nil
	})).Return(nil).Once()

	got, err := svc.Create(context.Background(), in.CreateHouseholdCommand{OwnerName: "Budi", Address: "Jl. Mawar 1"})

	require.NoError(t, err)
	assert.Equal(t, "Budi", got.OwnerName)
	assert.Equal(t, "Jl. Mawar 1", got.Address)
	assert.Equal(t, now, got.CreatedAt)
	assert.Equal(t, now, got.UpdatedAt)
	assert.NotEqual(t, uuid.Nil, got.ID)
	repo.AssertExpectations(t)
}

func TestHouseholdService_List(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	items := []domain.Household{
		{ID: uuid.New(), OwnerName: "Budi", Address: "Jl. Mawar 1", CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New(), OwnerName: "Siti", Address: "Jl. Melati 2", CreatedAt: now, UpdatedAt: now},
	}
	repo := new(repomock.HouseholdRepository)
	svc := service.NewHouseholdService(repo, fixedClock{now: now})

	params := pagination.Params{Page: 2, PerPage: 10}
	repo.On("List", mock.Anything, 10, 10).Return(items, nil).Once()
	repo.On("Count", mock.Anything).Return(42, nil).Once()

	got, total, err := svc.List(context.Background(), params)

	require.NoError(t, err)
	assert.Equal(t, items, got)
	assert.Equal(t, 42, total)
	repo.AssertExpectations(t)
}

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
			svc := service.NewHouseholdService(repo, fixedClock{now: now})
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
			svc := service.NewHouseholdService(repo, fixedClock{now: time.Now()})
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
