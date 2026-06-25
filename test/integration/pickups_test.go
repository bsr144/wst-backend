//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func pkBaseURL() string {
	return os.Getenv("REDGREEN_BASE_URL")
}

func pkSkipUnlessBaseURL(t *testing.T) {
	t.Helper()
	if pkBaseURL() == "" {
		t.Skip("REDGREEN_BASE_URL not set")
	}
}

func pkDoJSON(method, path string, body interface{}) (*http.Response, []byte) {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, pkBaseURL()+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func pkDoRaw(method, path, body string) (*http.Response, []byte) {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, pkBaseURL()+path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func pkPostPickupJSON429(body interface{}) (*http.Response, []byte) {
	var resp *http.Response
	var b []byte
	for attempt := 0; attempt < 16; attempt++ {
		resp, b = pkDoJSON("POST", "/api/pickups", body)
		if resp == nil || resp.StatusCode != 429 {
			break
		}
		time.Sleep(750 * time.Millisecond)
	}
	return resp, b
}

func pkPostPickupRaw429(body string) (*http.Response, []byte) {
	var resp *http.Response
	var b []byte
	for attempt := 0; attempt < 16; attempt++ {
		resp, b = pkDoRaw("POST", "/api/pickups", body)
		if resp == nil || resp.StatusCode != 429 {
			break
		}
		time.Sleep(750 * time.Millisecond)
	}
	return resp, b
}

func pkCreateHousehold(t *testing.T) string {
	t.Helper()
	resp, body := pkDoJSON("POST", "/api/households", map[string]string{
		"owner_name": "RedGreen Pickup HH " + t.Name(),
		"address":    "1 Pickup Test Lane",
	})
	if resp == nil || resp.StatusCode != 201 {
		t.Fatalf("pkCreateHousehold: got status %v body %s", resp, body)
	}
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("pkCreateHousehold parse: %v body %s", err, body)
	}
	return env.Data.ID
}

func pkCreatePickup(t *testing.T, householdID, pickupType string, safetyCheck *bool) string {
	t.Helper()
	payload := map[string]interface{}{
		"household_id": householdID,
		"type":         pickupType,
	}
	if safetyCheck != nil {
		payload["safety_check"] = *safetyCheck
	}
	var resp *http.Response
	var body []byte
	for attempt := 0; attempt < 12; attempt++ {
		resp, body = pkDoJSON("POST", "/api/pickups", payload)
		if resp == nil || resp.StatusCode != 429 {
			break
		}
		time.Sleep(750 * time.Millisecond)
	}
	if resp == nil || resp.StatusCode != 201 {
		t.Fatalf("pkCreatePickup(%s,%s): got status %v body %s", householdID, pickupType, resp, body)
	}
	var env struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("pkCreatePickup parse: %v body %s", err, body)
	}
	return env.Data.ID
}

func pkSchedulePickup(t *testing.T, pickupID string) {
	t.Helper()
	resp, body := pkDoJSON("PUT", "/api/pickups/"+pickupID+"/schedule", map[string]string{
		"pickup_date": "2026-07-15T09:00:00Z",
	})
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("pkSchedulePickup(%s): got %v body %s", pickupID, resp, body)
	}
}

func pkBoolPtr(v bool) *bool { return &v }

func pkGetErrorCode(body []byte) string {
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &env)
	return env.Error.Code
}

func pkAssertRequestID(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.Header.Get("X-Request-Id") == "" {
		t.Errorf("missing X-Request-Id response header")
	}
}

func TestRedGreenPickup_Create_Green(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	types := []struct {
		pickupType  string
		safetyCheck *bool
	}{
		{"organic", nil},
		{"plastic", nil},
		{"paper", nil},
		{"electronic", pkBoolPtr(true)},
		{"electronic", pkBoolPtr(false)},
	}

	for _, tc := range types {
		name := tc.pickupType
		if tc.safetyCheck != nil {
			name = fmt.Sprintf("%s_safety_%v", tc.pickupType, *tc.safetyCheck)
		}
		t.Run(name, func(t *testing.T) {
			hhID := pkCreateHousehold(t)
			payload := map[string]interface{}{
				"household_id": hhID,
				"type":         tc.pickupType,
			}
			if tc.safetyCheck != nil {
				payload["safety_check"] = *tc.safetyCheck
			}
			resp, body := pkPostPickupJSON429(payload)
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != 201 {
				t.Fatalf("want 201 got %d body %s", resp.StatusCode, body)
			}
			var env struct {
				Data struct {
					Status string `json:"status"`
					Type   string `json:"type"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &env); err != nil {
				t.Fatalf("parse: %v body %s", err, body)
			}
			if env.Data.Status != "pending" {
				t.Errorf("want status=pending got %s", env.Data.Status)
			}
			if env.Data.Type != tc.pickupType {
				t.Errorf("want type=%s got %s", tc.pickupType, env.Data.Type)
			}
		})
	}
}

func TestRedGreenPickup_Create_Red_Validation(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)

	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid_type_metal",
			body:       fmt.Sprintf(`{"household_id":%q,"type":"metal"}`, hhID),
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "missing_type",
			body:       fmt.Sprintf(`{"household_id":%q}`, hhID),
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "non_uuid_household_id",
			body:       `{"household_id":"not-a-uuid","type":"organic"}`,
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "unknown_household_id",
			body:       `{"household_id":"00000000-0000-0000-0000-000000000001","type":"organic"}`,
			wantStatus: 404,
			wantCode:   "HOUSEHOLD_NOT_FOUND",
		},
		{
			name:       "electronic_missing_safety_check",
			body:       fmt.Sprintf(`{"household_id":%q,"type":"electronic"}`, hhID),
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "malformed_json",
			body:       `{bad json}`,
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := pkPostPickupRaw429(tc.body)
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("want %d got %d body %s", tc.wantStatus, resp.StatusCode, body)
			}
			if code := pkGetErrorCode(body); code != tc.wantCode {
				t.Errorf("want code=%s got %s body %s", tc.wantCode, code, body)
			}
		})
	}
}

func TestRedGreenPickup_Create_Rule1_HouseholdHasPendingPayment(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	pickupID := pkCreatePickup(t, hhID, "plastic", nil)

	resp, body := pkDoJSON("POST", "/api/payments", map[string]string{
		"household_id": hhID,
		"waste_id":     pickupID,
	})
	if resp == nil || resp.StatusCode != 201 {
		t.Fatalf("create payment: got %v body %s", resp, body)
	}

	resp2, body2 := pkPostPickupJSON429(map[string]string{
		"household_id": hhID,
		"type":         "organic",
	})
	if resp2 == nil {
		t.Fatal("no response")
	}
	pkAssertRequestID(t, resp2)
	if resp2.StatusCode != 409 {
		t.Errorf("rule 1: want 409 got %d body %s", resp2.StatusCode, body2)
	}
	if code := pkGetErrorCode(body2); code != "HOUSEHOLD_HAS_PENDING_PAYMENT" {
		t.Errorf("rule 1: want HOUSEHOLD_HAS_PENDING_PAYMENT got %s", code)
	}
}

func TestRedGreenPickup_List_Green(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	_ = pkCreatePickup(t, hhID, "paper", nil)

	t.Run("all", func(t *testing.T) {
		resp, body := pkDoJSON("GET", "/api/pickups", nil)
		if resp == nil {
			t.Fatal("no response")
		}
		pkAssertRequestID(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
		}
		var env struct {
			Data []interface{} `json:"data"`
			Meta struct {
				Page    int `json:"page"`
				PerPage int `json:"per_page"`
				Total   int `json:"total"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("parse: %v body %s", err, body)
		}
		if env.Meta.Page == 0 || env.Meta.PerPage == 0 {
			t.Errorf("meta missing page/per_page body %s", body)
		}
	})

	t.Run("filter_by_status_pending", func(t *testing.T) {
		resp, body := pkDoJSON("GET", "/api/pickups?status=pending", nil)
		if resp == nil {
			t.Fatal("no response")
		}
		if resp.StatusCode != 200 {
			t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
		}
		var env struct {
			Data []struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("parse: %v body %s", err, body)
		}
		for _, d := range env.Data {
			if d.Status != "pending" {
				t.Errorf("filter status=pending returned item with status=%s", d.Status)
			}
		}
	})

	t.Run("filter_by_household_id", func(t *testing.T) {
		resp, body := pkDoJSON("GET", "/api/pickups?household_id="+hhID, nil)
		if resp == nil {
			t.Fatal("no response")
		}
		if resp.StatusCode != 200 {
			t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
		}
		var env struct {
			Data []struct {
				HouseholdID string `json:"household_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("parse: %v body %s", err, body)
		}
		found := false
		for _, d := range env.Data {
			if d.HouseholdID == hhID {
				found = true
			}
		}
		if !found {
			t.Errorf("filter household_id=%s: household not found in results body %s", hhID, body)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		resp, body := pkDoJSON("GET", "/api/pickups?page=1&per_page=2", nil)
		if resp == nil {
			t.Fatal("no response")
		}
		if resp.StatusCode != 200 {
			t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
		}
		var env struct {
			Data []interface{} `json:"data"`
			Meta struct {
				Page    int `json:"page"`
				PerPage int `json:"per_page"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("parse: %v body %s", err, body)
		}
		if env.Meta.PerPage != 2 {
			t.Errorf("want per_page=2 got %d", env.Meta.PerPage)
		}
		if len(env.Data) > 2 {
			t.Errorf("want <=2 items got %d", len(env.Data))
		}
	})
}

func TestRedGreenPickup_List_Red_Validation(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "bad_status_enum",
			path:       "/api/pickups?status=invalid",
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "non_uuid_household_id",
			path:       "/api/pickups?household_id=not-a-uuid",
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := pkDoJSON("GET", tc.path, nil)
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("want %d got %d body %s", tc.wantStatus, resp.StatusCode, body)
			}
			if code := pkGetErrorCode(body); code != tc.wantCode {
				t.Errorf("want code=%s got %s body %s", tc.wantCode, code, body)
			}
		})
	}
}

func TestRedGreenPickup_Schedule_Green(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	pickupID := pkCreatePickup(t, hhID, "plastic", nil)

	resp, body := pkDoJSON("PUT", "/api/pickups/"+pickupID+"/schedule", map[string]string{
		"pickup_date": "2026-07-15T09:00:00Z",
	})
	if resp == nil {
		t.Fatal("no response")
	}
	pkAssertRequestID(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
	}
	var env struct {
		Data struct {
			Status     string  `json:"status"`
			PickupDate *string `json:"pickup_date"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("parse: %v body %s", err, body)
	}
	if env.Data.Status != "scheduled" {
		t.Errorf("want status=scheduled got %s", env.Data.Status)
	}
	if env.Data.PickupDate == nil || *env.Data.PickupDate == "" {
		t.Errorf("want pickup_date set, got nil/empty")
	}
}

func TestRedGreenPickup_Schedule_Green_Electronic_Safety(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	pickupID := pkCreatePickup(t, hhID, "electronic", pkBoolPtr(true))

	resp, body := pkDoJSON("PUT", "/api/pickups/"+pickupID+"/schedule", map[string]string{
		"pickup_date": "2026-07-20T09:00:00Z",
	})
	if resp == nil {
		t.Fatal("no response")
	}
	pkAssertRequestID(t, resp)
	if resp.StatusCode != 200 {
		t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
	}
}

func TestRedGreenPickup_Schedule_Red(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	pendingID := pkCreatePickup(t, hhID, "plastic", nil)
	pkSchedulePickup(t, pendingID)

	hhID2 := pkCreateHousehold(t)
	pendingID2 := pkCreatePickup(t, hhID2, "organic", nil)

	hhID3 := pkCreateHousehold(t)
	elecNoSafetyID := pkCreatePickup(t, hhID3, "electronic", pkBoolPtr(false))

	cases := []struct {
		name       string
		pickupID   string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "non_uuid_id",
			pickupID:   "not-a-uuid",
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "unknown_id",
			pickupID:   "00000000-0000-0000-0000-000000000001",
			wantStatus: 404,
			wantCode:   "PICKUP_NOT_FOUND",
		},
		{
			name:       "rule2_already_scheduled",
			pickupID:   pendingID,
			wantStatus: 409,
			wantCode:   "PICKUP_NOT_PENDING",
		},
		{
			name:       "missing_pickup_date",
			pickupID:   pendingID2,
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "rule3_electronic_safety_false",
			pickupID:   elecNoSafetyID,
			wantStatus: 422,
			wantCode:   "SAFETY_CHECK_REQUIRED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp *http.Response
			var body []byte
			if tc.name == "missing_pickup_date" {
				resp, body = pkDoJSON("PUT", "/api/pickups/"+tc.pickupID+"/schedule", map[string]string{})
			} else {
				resp, body = pkDoJSON("PUT", "/api/pickups/"+tc.pickupID+"/schedule", map[string]string{
					"pickup_date": "2026-07-15T09:00:00Z",
				})
			}
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("want %d got %d body %s", tc.wantStatus, resp.StatusCode, body)
			}
			if code := pkGetErrorCode(body); code != tc.wantCode {
				t.Errorf("want code=%s got %s body %s", tc.wantCode, code, body)
			}
		})
	}
}

func TestRedGreenPickup_Complete_Green_And_Rule5(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	cases := []struct {
		pickupType     string
		expectedAmount string
	}{
		{"organic", "10000.00"},
		{"plastic", "10000.00"},
		{"electronic", "50000.00"},
	}

	for _, tc := range cases {
		t.Run(tc.pickupType, func(t *testing.T) {
			hhID := pkCreateHousehold(t)
			var pickupID string
			if tc.pickupType == "electronic" {
				pickupID = pkCreatePickup(t, hhID, tc.pickupType, pkBoolPtr(true))
			} else {
				pickupID = pkCreatePickup(t, hhID, tc.pickupType, nil)
			}
			pkSchedulePickup(t, pickupID)

			resp, body := pkDoRaw("PUT", "/api/pickups/"+pickupID+"/complete", "")
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != 200 {
				t.Fatalf("complete: want 200 got %d body %s", resp.StatusCode, body)
			}
			var env struct {
				Data struct {
					Status string `json:"status"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &env); err != nil {
				t.Fatalf("parse complete: %v body %s", err, body)
			}
			if env.Data.Status != "completed" {
				t.Errorf("want status=completed got %s", env.Data.Status)
			}

			payResp, payBody := pkDoJSON("GET", "/api/payments?household_id="+hhID+"&status=pending", nil)
			if payResp == nil {
				t.Fatal("no payment response")
			}
			if payResp.StatusCode != 200 {
				t.Fatalf("payments: want 200 got %d body %s", payResp.StatusCode, payBody)
			}
			var payEnv struct {
				Data []struct {
					WasteID string `json:"waste_id"`
					Amount  string `json:"amount"`
					Status  string `json:"status"`
				} `json:"data"`
			}
			if err := json.Unmarshal(payBody, &payEnv); err != nil {
				t.Fatalf("parse payments: %v body %s", err, payBody)
			}
			found := false
			for _, p := range payEnv.Data {
				if p.WasteID == pickupID {
					found = true
					if p.Amount != tc.expectedAmount {
						t.Errorf("rule 5 amount: want %s got %s", tc.expectedAmount, p.Amount)
					}
					if p.Status != "pending" {
						t.Errorf("rule 5 status: want pending got %s", p.Status)
					}
				}
			}
			if !found {
				t.Errorf("rule 5: no payment found for pickup %s in household %s body %s", pickupID, hhID, payBody)
			}
		})
	}
}

func TestRedGreenPickup_Complete_Red(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	pendingID := pkCreatePickup(t, hhID, "organic", nil)

	cases := []struct {
		name       string
		pickupID   string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "non_uuid_id",
			pickupID:   "not-a-uuid",
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "unknown_id",
			pickupID:   "00000000-0000-0000-0000-000000000001",
			wantStatus: 404,
			wantCode:   "PICKUP_NOT_FOUND",
		},
		{
			name:       "still_pending",
			pickupID:   pendingID,
			wantStatus: 409,
			wantCode:   "PICKUP_NOT_SCHEDULED",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := pkDoRaw("PUT", "/api/pickups/"+tc.pickupID+"/complete", "")
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("want %d got %d body %s", tc.wantStatus, resp.StatusCode, body)
			}
			if code := pkGetErrorCode(body); code != tc.wantCode {
				t.Errorf("want code=%s got %s body %s", tc.wantCode, code, body)
			}
		})
	}
}

func TestRedGreenPickup_Cancel_Green(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	t.Run("cancel_pending", func(t *testing.T) {
		hhID := pkCreateHousehold(t)
		pickupID := pkCreatePickup(t, hhID, "organic", nil)

		resp, body := pkDoRaw("PUT", "/api/pickups/"+pickupID+"/cancel", "")
		if resp == nil {
			t.Fatal("no response")
		}
		pkAssertRequestID(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
		}
		var env struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("parse: %v body %s", err, body)
		}
		if env.Data.Status != "canceled" {
			t.Errorf("want status=canceled got %s", env.Data.Status)
		}
	})

	t.Run("cancel_scheduled", func(t *testing.T) {
		hhID := pkCreateHousehold(t)
		pickupID := pkCreatePickup(t, hhID, "plastic", nil)
		pkSchedulePickup(t, pickupID)

		resp, body := pkDoRaw("PUT", "/api/pickups/"+pickupID+"/cancel", "")
		if resp == nil {
			t.Fatal("no response")
		}
		pkAssertRequestID(t, resp)
		if resp.StatusCode != 200 {
			t.Fatalf("want 200 got %d body %s", resp.StatusCode, body)
		}
		var env struct {
			Data struct {
				Status string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("parse: %v body %s", err, body)
		}
		if env.Data.Status != "canceled" {
			t.Errorf("want status=canceled got %s", env.Data.Status)
		}
	})
}

func TestRedGreenPickup_Cancel_Red(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)
	pickupID := pkCreatePickup(t, hhID, "organic", nil)
	pkSchedulePickup(t, pickupID)

	resp, body := pkDoRaw("PUT", "/api/pickups/"+pickupID+"/complete", "")
	if resp == nil || resp.StatusCode != 200 {
		t.Fatalf("setup complete: got %v body %s", resp, body)
	}

	hhID2 := pkCreateHousehold(t)
	canceledID := pkCreatePickup(t, hhID2, "paper", nil)
	resp2, body2 := pkDoRaw("PUT", "/api/pickups/"+canceledID+"/cancel", "")
	if resp2 == nil || resp2.StatusCode != 200 {
		t.Fatalf("setup cancel: got %v body %s", resp2, body2)
	}

	cases := []struct {
		name       string
		pickupID   string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "non_uuid_id",
			pickupID:   "not-a-uuid",
			wantStatus: 400,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "unknown_id",
			pickupID:   "00000000-0000-0000-0000-000000000001",
			wantStatus: 404,
			wantCode:   "PICKUP_NOT_FOUND",
		},
		{
			name:       "completed_not_cancelable",
			pickupID:   pickupID,
			wantStatus: 409,
			wantCode:   "PICKUP_NOT_CANCELABLE",
		},
		{
			name:       "already_canceled",
			pickupID:   canceledID,
			wantStatus: 409,
			wantCode:   "PICKUP_NOT_CANCELABLE",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := pkDoRaw("PUT", "/api/pickups/"+tc.pickupID+"/cancel", "")
			if resp == nil {
				t.Fatal("no response")
			}
			pkAssertRequestID(t, resp)
			if resp.StatusCode != tc.wantStatus {
				t.Errorf("want %d got %d body %s", tc.wantStatus, resp.StatusCode, body)
			}
			if code := pkGetErrorCode(body); code != tc.wantCode {
				t.Errorf("want code=%s got %s body %s", tc.wantCode, code, body)
			}
		})
	}
}

func TestRedGreenPickup_RateLimit_429_Hammer(t *testing.T) {
	pkSkipUnlessBaseURL(t)

	hhID := pkCreateHousehold(t)

	const total = 90
	var got429 atomic.Int32

	client := &http.Client{Timeout: 10 * time.Second}
	for i := 0; i < total; i++ {
		payload := fmt.Sprintf(`{"household_id":%q,"type":"organic"}`, hhID)
		req, _ := http.NewRequest("POST", pkBaseURL()+"/api/pickups", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			code := pkGetErrorCode(b)
			if code != "RATE_LIMITED" {
				t.Errorf("429 body: want code=RATE_LIMITED got %s body %s", code, b)
			}
			got429.Add(1)
		}
	}

	if got429.Load() == 0 {
		t.Errorf("hammer %d POST /api/pickups: got zero 429 responses; rate limiter may not be active", total)
	}
	t.Logf("hammer %d requests: %d returned 429 RATE_LIMITED", total, got429.Load())
}
