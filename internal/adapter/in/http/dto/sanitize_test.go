package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"wst-backend/internal/adapter/in/http/dto"
)

func TestCreateHouseholdRequest_Sanitize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		inName    string
		inAddress string
		wantName  string
		wantAddr  string
	}{
		{"trims outer whitespace", "  Budi  ", "  Jl. Mawar 1  ", "Budi", "Jl. Mawar 1"},
		{"collapses internal runs", "Budi    Santoso", "Jl.   Mawar    1", "Budi Santoso", "Jl. Mawar 1"},
		{"replaces null byte with space", "Bu\x00di", "Jl.\x00Mawar", "Bu di", "Jl. Mawar"},
		{"replaces newlines and tabs", "Budi\nSantoso", "Jl.\tMawar\r\n1", "Budi Santoso", "Jl. Mawar 1"},
		{"whitespace-only becomes empty", "   ", "\t\n ", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := dto.CreateHouseholdRequest{OwnerName: tc.inName, Address: tc.inAddress}
			req.Sanitize()
			assert.Equal(t, tc.wantName, req.OwnerName)
			assert.Equal(t, tc.wantAddr, req.Address)
		})
	}
}

func TestCreatePickupRequest_Sanitize(t *testing.T) {
	t.Parallel()
	req := dto.CreatePickupRequest{HouseholdID: "  abc\x00  ", Type: " organic\n"}
	req.Sanitize()
	assert.Equal(t, "abc", req.HouseholdID)
	assert.Equal(t, "organic", req.Type)
}

func TestCreatePaymentRequest_Sanitize(t *testing.T) {
	t.Parallel()
	req := dto.CreatePaymentRequest{HouseholdID: "\thh-1 ", WasteID: " w\r\n1 "}
	req.Sanitize()
	assert.Equal(t, "hh-1", req.HouseholdID)
	assert.Equal(t, "w 1", req.WasteID)
}

func TestListPickupsQuery_Sanitize(t *testing.T) {
	t.Parallel()
	q := dto.ListPickupsQuery{Status: " pending ", HouseholdID: "  id\x00 "}
	q.Sanitize()
	assert.Equal(t, "pending", q.Status)
	assert.Equal(t, "id", q.HouseholdID)
}

func TestListPaymentsQuery_Sanitize(t *testing.T) {
	t.Parallel()
	q := dto.ListPaymentsQuery{
		Status:      " paid ",
		HouseholdID: "  id ",
		DateFrom:    " 2026-06-01T00:00:00Z\n",
		DateTo:      "\t2026-06-30T00:00:00Z ",
	}
	q.Sanitize()
	assert.Equal(t, "paid", q.Status)
	assert.Equal(t, "id", q.HouseholdID)
	assert.Equal(t, "2026-06-01T00:00:00Z", q.DateFrom)
	assert.Equal(t, "2026-06-30T00:00:00Z", q.DateTo)
}
