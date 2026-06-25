//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func rgPKBase(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDGREEN_BASE_URL")
	if u == "" {
		t.Skip("REDGREEN_BASE_URL not set — skipping pickups red-green tests")
	}
	return strings.TrimRight(u, "/")
}

func rgPKDo(t *testing.T, method, url, contentType, body string) (*http.Response, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, url, err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return resp, m
}

func rgPKDoWithRetry429(t *testing.T, method, url, contentType, body string) (*http.Response, map[string]any) {
	t.Helper()
	for i := 0; i < 8; i++ {
		resp, m := rgPKDo(t, method, url, contentType, body)
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, m
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("still getting 429 after retries: %s %s", method, url)
	return nil, nil
}

func rgPKAssertErrorCode(t *testing.T, m map[string]any, want string) {
	t.Helper()
	errRaw, ok := m["error"]
	if !ok {
		t.Errorf("response missing 'error' key; got: %v", m)
		return
	}
	errMap, ok := errRaw.(map[string]any)
	if !ok {
		t.Errorf("error must be object, got %T", errRaw)
		return
	}
	code, _ := errMap["code"].(string)
	if code != want {
		t.Errorf("error.code: want %q, got %q", want, code)
	}
	if _, ok := errMap["message"]; !ok {
		t.Errorf("error.message missing")
	}
}

func rgPKCreateHousehold(t *testing.T, base, name, addr string) string {
	t.Helper()
	body := fmt.Sprintf(`{"owner_name":%q,"address":%q}`, name, addr)
	resp, m := rgPKDo(t, http.MethodPost, base+"/api/households", "application/json", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create household: want 201, got %d body=%v", resp.StatusCode, m)
	}
	data, _ := m["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("create household: data.id empty")
	}
	return id
}

func rgPKCreatePickup(t *testing.T, base, householdID, typ string, safetyCheck *bool) string {
	t.Helper()
	body := fmt.Sprintf(`{"household_id":%q,"type":%q`, householdID, typ)
	if safetyCheck != nil {
		if *safetyCheck {
			body += `,"safety_check":true`
		} else {
			body += `,"safety_check":false`
		}
	}
	body += `}`
	var resp *http.Response
	var m map[string]any
	for i := 0; i < 6; i++ {
		resp, m = rgPKDo(t, http.MethodPost, base+"/api/pickups", "application/json", body)
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create pickup type=%s: want 201, got %d body=%v", typ, resp.StatusCode, m)
	}
	data, _ := m["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("create pickup: data.id empty")
	}
	return id
}

func rgPKSchedule(t *testing.T, base, pickupID string) {
	t.Helper()
	body := `{"pickup_date":"2026-08-01T09:00:00Z"}`
	resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pickupID+"/schedule", "application/json", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("schedule pickup %s: want 200, got %d body=%v", pickupID, resp.StatusCode, m)
	}
}

func rgPKComplete(t *testing.T, base, pickupID string) {
	t.Helper()
	resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pickupID+"/complete", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete pickup %s: want 200, got %d body=%v", pickupID, resp.StatusCode, m)
	}
}

func rgPKPaymentsForHousehold(t *testing.T, base, householdID string) []map[string]any {
	t.Helper()
	resp, m := rgPKDo(t, http.MethodGet, base+"/api/payments?household_id="+householdID, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list payments: want 200, got %d", resp.StatusCode)
	}
	raw, _ := m["data"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if pm, ok := r.(map[string]any); ok {
			out = append(out, pm)
		}
	}
	return out
}

func boolPtr(b bool) *bool { return &b }

func TestPickupsRedGreen(t *testing.T) {
	base := rgPKBase(t)

	t.Run("G01_POST_organic_201_pending", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G01 Organic", "Jl. RGP G01 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"organic"}`, hhID))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%v", resp.StatusCode, m)
		}
		data, ok := m["data"].(map[string]any)
		if !ok {
			t.Fatal("data must be object")
		}
		for _, f := range []string{"id", "household_id", "type", "status", "created_at", "updated_at"} {
			if _, has := data[f]; !has {
				t.Errorf("data missing field %q", f)
			}
		}
		if status, _ := data["status"].(string); status != "pending" {
			t.Errorf("status: want pending, got %q", status)
		}
		if typ, _ := data["type"].(string); typ != "organic" {
			t.Errorf("type: want organic, got %q", typ)
		}
		if _, hasMeta := m["meta"]; hasMeta {
			t.Error("meta must NOT be present on create")
		}
		if resp.Header.Get("X-Request-Id") == "" {
			t.Error("X-Request-Id header missing")
		}
	})

	t.Run("G02_POST_plastic_201_pending", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G02 Plastic", "Jl. RGP G02 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"plastic"}`, hhID))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "pending" {
			t.Errorf("status: want pending, got %q", status)
		}
	})

	t.Run("G03_POST_paper_201_pending", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G03 Paper", "Jl. RGP G03 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"paper"}`, hhID))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "pending" {
			t.Errorf("status: want pending, got %q", status)
		}
	})

	t.Run("G04_POST_electronic_safety_true_201", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G04 Elec", "Jl. RGP G04 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"electronic","safety_check":true}`, hhID))
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("want 201, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "pending" {
			t.Errorf("status: want pending, got %q", status)
		}
		if typ, _ := data["type"].(string); typ != "electronic" {
			t.Errorf("type: want electronic, got %q", typ)
		}
	})

	t.Run("G05_GET_list_200_with_meta", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodGet, base+"/api/pickups", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if _, ok := m["data"].([]any); !ok {
			t.Fatal("data must be array")
		}
		meta, ok := m["meta"].(map[string]any)
		if !ok {
			t.Fatal("meta must be present and object on list")
		}
		for _, key := range []string{"page", "per_page", "total"} {
			if _, has := meta[key]; !has {
				t.Errorf("meta missing key %q", key)
			}
		}
	})

	t.Run("G06_GET_list_filter_status_pending", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodGet, base+"/api/pickups?status=pending", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		rows, _ := m["data"].([]any)
		for _, r := range rows {
			row, _ := r.(map[string]any)
			if s, _ := row["status"].(string); s != "pending" {
				t.Errorf("all rows must have status=pending, got %q", s)
			}
		}
	})

	t.Run("G07_GET_list_filter_household_id", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G07 Filter", "Jl. RGP G07 1")
		pkID := rgPKCreatePickup(t, base, hhID, "organic", nil)
		resp, m := rgPKDo(t, http.MethodGet, base+"/api/pickups?household_id="+hhID, "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		rows, _ := m["data"].([]any)
		found := false
		for _, r := range rows {
			row, _ := r.(map[string]any)
			if id, _ := row["id"].(string); id == pkID {
				found = true
			}
			if hh, _ := row["household_id"].(string); hh != hhID {
				t.Errorf("all rows must belong to household %s, got %s", hhID, hh)
			}
		}
		if !found {
			t.Errorf("created pickup %s not found in filtered list", pkID)
		}
	})

	t.Run("G08_PUT_schedule_pending_to_scheduled_200", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G08 Sched", "Jl. RGP G08 1")
		pkID := rgPKCreatePickup(t, base, hhID, "plastic", nil)
		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/schedule",
			"application/json", `{"pickup_date":"2026-09-01T09:00:00Z"}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "scheduled" {
			t.Errorf("status: want scheduled, got %q", status)
		}
		if pd, _ := data["pickup_date"].(string); pd == "" {
			t.Error("pickup_date must be set after schedule")
		}
	})

	t.Run("G09_PUT_complete_scheduled_200_and_rule5_payment_standard", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G09 Complete", "Jl. RGP G09 1")
		pkID := rgPKCreatePickup(t, base, hhID, "paper", nil)
		rgPKSchedule(t, base, pkID)

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/complete", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "completed" {
			t.Errorf("status: want completed, got %q", status)
		}

		payments := rgPKPaymentsForHousehold(t, base, hhID)
		var found bool
		for _, p := range payments {
			if wid, _ := p["waste_id"].(string); wid == pkID {
				found = true
				if s, _ := p["status"].(string); s != "pending" {
					t.Errorf("rule5: auto-payment status want pending, got %q", s)
				}
				if amt, _ := p["amount"].(string); amt != "10000.00" {
					t.Errorf("rule5: paper amount want 10000.00, got %q", amt)
				}
			}
		}
		if !found {
			t.Error("rule5: no payment found for completed pickup")
		}
	})

	t.Run("G10_PUT_complete_electronic_rule5_payment_electronic", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G10 ElecComplete", "Jl. RGP G10 1")
		pkID := rgPKCreatePickup(t, base, hhID, "electronic", boolPtr(true))
		rgPKSchedule(t, base, pkID)
		rgPKComplete(t, base, pkID)

		payments := rgPKPaymentsForHousehold(t, base, hhID)
		var found bool
		for _, p := range payments {
			if wid, _ := p["waste_id"].(string); wid == pkID {
				found = true
				if s, _ := p["status"].(string); s != "pending" {
					t.Errorf("rule5 electronic: payment status want pending, got %q", s)
				}
				if amt, _ := p["amount"].(string); amt != "50000.00" {
					t.Errorf("rule5 electronic: amount want 50000.00, got %q", amt)
				}
			}
		}
		if !found {
			t.Error("rule5: no payment found for completed electronic pickup")
		}
	})

	t.Run("G11_PUT_cancel_pending_200_canceled", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP G11 Cancel", "Jl. RGP G11 1")
		pkID := rgPKCreatePickup(t, base, hhID, "organic", nil)
		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/cancel", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "canceled" {
			t.Errorf("status: want canceled, got %q", status)
		}
	})

	t.Run("R01_POST_missing_type_400_validation_error", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R01", "Jl. RGP R01 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q}`, hhID))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R02_POST_bad_type_enum_400_validation_error", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R02", "Jl. RGP R02 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"garbage"}`, hhID))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R03_POST_electronic_without_safety_check_400", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R03", "Jl. RGP R03 1")
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"electronic"}`, hhID))
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R04_POST_non_uuid_household_id_400", func(t *testing.T) {
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			`{"household_id":"not-a-uuid","type":"organic"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R05_POST_unknown_household_uuid_404", func(t *testing.T) {
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			`{"household_id":"00000000-0000-0000-0000-000000000099","type":"organic"}`)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("want 404, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "HOUSEHOLD_NOT_FOUND")
	})

	t.Run("R06_POST_malformed_json_400_not_500", func(t *testing.T) {
		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			`{"household_id":bad json}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400 (not 500), got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R07_POST_rule1_household_has_pending_payment_409", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R07 Rule1", "Jl. RGP R07 1")
		pkID := rgPKCreatePickup(t, base, hhID, "paper", nil)
		rgPKSchedule(t, base, pkID)
		rgPKComplete(t, base, pkID)

		payments := rgPKPaymentsForHousehold(t, base, hhID)
		hasPending := false
		for _, p := range payments {
			if s, _ := p["status"].(string); s == "pending" {
				hasPending = true
			}
		}
		if !hasPending {
			t.Fatal("setup: household must have a pending payment before rule-1 check")
		}

		resp, m := rgPKDoWithRetry429(t, http.MethodPost, base+"/api/pickups", "application/json",
			fmt.Sprintf(`{"household_id":%q,"type":"plastic"}`, hhID))
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("rule1: want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "HOUSEHOLD_HAS_PENDING_PAYMENT")
	})

	t.Run("R08_GET_list_bad_status_enum_400", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodGet, base+"/api/pickups?status=invalid", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R09_GET_list_bad_household_id_uuid_400", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodGet, base+"/api/pickups?household_id=not-a-uuid", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R10_PUT_schedule_non_pending_409_rule2", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R10 Rule2", "Jl. RGP R10 1")
		pkID := rgPKCreatePickup(t, base, hhID, "plastic", nil)
		rgPKSchedule(t, base, pkID)

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/schedule",
			"application/json", `{"pickup_date":"2026-09-01T09:00:00Z"}`)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("rule2: want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_PENDING")
	})

	t.Run("R11_PUT_schedule_completed_409_rule2", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R11 Rule2B", "Jl. RGP R11 1")
		pkID := rgPKCreatePickup(t, base, hhID, "plastic", nil)
		rgPKSchedule(t, base, pkID)
		rgPKComplete(t, base, pkID)

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/schedule",
			"application/json", `{"pickup_date":"2026-09-01T09:00:00Z"}`)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("rule2 completed: want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_PENDING")
	})

	t.Run("R12_PUT_schedule_electronic_safety_false_422_rule3", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R12 Rule3", "Jl. RGP R12 1")
		pkID := rgPKCreatePickup(t, base, hhID, "electronic", boolPtr(false))

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/schedule",
			"application/json", `{"pickup_date":"2026-09-01T09:00:00Z"}`)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("rule3: want 422, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "SAFETY_CHECK_REQUIRED")
	})

	t.Run("R13_PUT_schedule_missing_pickup_date_400", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R13", "Jl. RGP R13 1")
		pkID := rgPKCreatePickup(t, base, hhID, "organic", nil)

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/schedule",
			"application/json", `{}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R14_PUT_schedule_unknown_uuid_404", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodPut,
			base+"/api/pickups/00000000-0000-0000-0000-000000000099/schedule",
			"application/json", `{"pickup_date":"2026-09-01T09:00:00Z"}`)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("want 404, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_FOUND")
	})

	t.Run("R15_PUT_schedule_non_uuid_id_400", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/not-a-uuid/schedule",
			"application/json", `{"pickup_date":"2026-09-01T09:00:00Z"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R16_PUT_complete_pending_not_scheduled_409", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R16", "Jl. RGP R16 1")
		pkID := rgPKCreatePickup(t, base, hhID, "plastic", nil)

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/complete", "", "")
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_SCHEDULED")
	})

	t.Run("R17_PUT_complete_unknown_uuid_404", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodPut,
			base+"/api/pickups/00000000-0000-0000-0000-000000000099/complete", "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("want 404, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_FOUND")
	})

	t.Run("R18_PUT_complete_non_uuid_id_400", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/not-a-uuid/complete", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R19_PUT_cancel_already_canceled_409", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R19", "Jl. RGP R19 1")
		pkID := rgPKCreatePickup(t, base, hhID, "organic", nil)
		resp1, _ := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/cancel", "", "")
		if resp1.StatusCode != http.StatusOK {
			t.Fatalf("first cancel: want 200, got %d", resp1.StatusCode)
		}

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/cancel", "", "")
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_CANCELABLE")
	})

	t.Run("R20_PUT_cancel_completed_409", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R20", "Jl. RGP R20 1")
		pkID := rgPKCreatePickup(t, base, hhID, "paper", nil)
		rgPKSchedule(t, base, pkID)
		rgPKComplete(t, base, pkID)

		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/"+pkID+"/cancel", "", "")
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_CANCELABLE")
	})

	t.Run("R21_PUT_cancel_unknown_uuid_404", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodPut,
			base+"/api/pickups/00000000-0000-0000-0000-000000000099/cancel", "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("want 404, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "PICKUP_NOT_FOUND")
	})

	t.Run("R22_PUT_cancel_non_uuid_id_400", func(t *testing.T) {
		resp, m := rgPKDo(t, http.MethodPut, base+"/api/pickups/not-a-uuid/cancel", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPKAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R23_POST_rate_limit_burst_429_rate_limited", func(t *testing.T) {
		hhID := rgPKCreateHousehold(t, base, "RGP R23 RateLimit", "Jl. RGP R23 1")
		body := fmt.Sprintf(`{"household_id":%q,"type":"plastic"}`, hhID)

		rateLimited := 0
		for i := 0; i < 25; i++ {
			resp, _ := rgPKDo(t, http.MethodPost, base+"/api/pickups", "application/json", body)
			if resp.StatusCode == http.StatusTooManyRequests {
				rateLimited++
			}
		}
		if rateLimited == 0 {
			t.Error("rate limit: expected at least one 429 RATE_LIMITED in 25 rapid requests")
		}
	})
}
