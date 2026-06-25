//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func buildRichHistoryHousehold(t *testing.T, base string) (hhID, donePickupID, cancelPickupID, schedPickupID string) {
	t.Helper()

	status, body, _ := doPost(t, base+"/api/households",
		`{"owner_name":"RG Rich History","address":"Jl. Rich 1"}`)
	if status != 201 {
		t.Fatalf("create household: want 201, got %d", status)
	}
	data, _ := body["data"].(map[string]interface{})
	hhID, _ = data["id"].(string)
	if hhID == "" {
		t.Fatal("household id empty")
	}

	status, body, _ = doPost(t, base+"/api/pickups",
		fmt.Sprintf(`{"household_id":%q,"type":"paper"}`, hhID))
	if status != 201 {
		t.Fatalf("create paper pickup: want 201, got %d", status)
	}
	d, _ := body["data"].(map[string]interface{})
	donePickupID, _ = d["id"].(string)

	status, body, _ = doPost(t, base+"/api/pickups",
		fmt.Sprintf(`{"household_id":%q,"type":"plastic"}`, hhID))
	if status != 201 {
		t.Fatalf("create plastic pickup: want 201, got %d", status)
	}
	d, _ = body["data"].(map[string]interface{})
	cancelPickupID, _ = d["id"].(string)

	status, body, _ = doPost(t, base+"/api/pickups",
		fmt.Sprintf(`{"household_id":%q,"type":"electronic","safety_check":true}`, hhID))
	if status != 201 {
		t.Fatalf("create electronic pickup: want 201, got %d", status)
	}
	d, _ = body["data"].(map[string]interface{})
	schedPickupID, _ = d["id"].(string)

	status, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/schedule", base, schedPickupID),
		`{"pickup_date":"2026-09-01T10:00:00Z"}`)
	if status != 200 {
		t.Fatalf("schedule electronic: want 200, got %d", status)
	}

	status, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/cancel", base, cancelPickupID), "")
	if status != 200 {
		t.Fatalf("cancel plastic: want 200, got %d", status)
	}

	status, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/schedule", base, donePickupID),
		`{"pickup_date":"2026-09-01T10:00:00Z"}`)
	if status != 200 {
		t.Fatalf("schedule paper: want 200, got %d", status)
	}

	status, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/complete", base, donePickupID), "")
	if status != 200 {
		t.Fatalf("complete paper: want 200, got %d", status)
	}

	return hhID, donePickupID, cancelPickupID, schedPickupID
}

func buildFreshCleanHousehold(t *testing.T, base string) string {
	t.Helper()

	status, body, _ := doPost(t, base+"/api/households",
		`{"owner_name":"RG Empty Hist","address":"Jl. Kosong 1"}`)
	if status != 201 {
		t.Fatalf("create clean household: want 201, got %d", status)
	}
	data, _ := body["data"].(map[string]interface{})
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("clean household id empty")
	}
	return id
}

func doMethod(t *testing.T, method, url, jsonBody string) (int, map[string]interface{}, http.Header) {
	t.Helper()
	var bodyReader io.Reader
	if jsonBody != "" {
		bodyReader = strings.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("new %s request: %v", method, err)
	}
	if jsonBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]interface{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &body)
	}
	return resp.StatusCode, body, resp.Header
}

func parseFloatStr(s string) (float64, bool) {
	var f float64
	n, _ := fmt.Sscanf(s, "%f", &f)
	return f, n == 1
}

func has2dp(s string) bool {
	idx := strings.Index(s, ".")
	if idx < 0 {
		return false
	}
	return len(s)-idx-1 == 2
}

func TestReportsRedGreen_WasteSummary(t *testing.T) {
	base := redgreenBaseURL(t)

	t.Run("green_canonical_shape_and_envelope", func(t *testing.T) {
		status, body, headers := doGet(t, base+"/api/reports/waste-summary")
		if status != 200 {
			t.Fatalf("want 200, got %d body=%v", status, body)
		}
		if xrid := headers.Get("X-Request-Id"); xrid == "" {
			t.Error("X-Request-Id header missing")
		}
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("body.data missing or not object: %v", body)
		}
		_, hasCounts := data["counts"]
		if !hasCounts {
			t.Error("data.counts missing")
		}
		_, hasTotal := data["total"]
		if !hasTotal {
			t.Error("data.total missing")
		}
		if _, hasMeta := body["meta"]; hasMeta {
			t.Error("meta must not be present on non-list endpoint")
		}
	})

	t.Run("green_total_equals_sum_counts_invariant", func(t *testing.T) {
		_, body, _ := doGet(t, base+"/api/reports/waste-summary")
		data, _ := body["data"].(map[string]interface{})
		counts, _ := data["counts"].([]interface{})
		total, _ := data["total"].(float64)

		var sum float64
		for _, ci := range counts {
			c, _ := ci.(map[string]interface{})
			cnt, _ := c["count"].(float64)
			sum += cnt
		}
		if total != sum {
			t.Errorf("invariant: total=%v != sum(counts)=%v", total, sum)
		}
		if total < 1 {
			t.Errorf("total should be >= 1, got %v (service has fixtures)", total)
		}
	})

	t.Run("green_enum_values_all_valid", func(t *testing.T) {
		_, body, _ := doGet(t, base+"/api/reports/waste-summary")
		data, _ := body["data"].(map[string]interface{})
		counts, _ := data["counts"].([]interface{})

		validTypes := map[string]bool{"organic": true, "plastic": true, "paper": true, "electronic": true}
		validStatuses := map[string]bool{"pending": true, "scheduled": true, "completed": true, "canceled": true}
		for i, ci := range counts {
			c, ok := ci.(map[string]interface{})
			if !ok {
				t.Errorf("counts[%d] not object", i)
				continue
			}
			typ, _ := c["type"].(string)
			sts, _ := c["status"].(string)
			cnt, _ := c["count"].(float64)
			if !validTypes[typ] {
				t.Errorf("counts[%d].type invalid: %q", i, typ)
			}
			if !validStatuses[sts] {
				t.Errorf("counts[%d].status invalid: %q", i, sts)
			}
			if cnt < 0 {
				t.Errorf("counts[%d].count negative: %v", i, cnt)
			}
		}
	})

	t.Run("green_stray_query_params_ignored", func(t *testing.T) {
		status, body, _ := doGet(t, base+"/api/reports/waste-summary?foo=bar&type=organic")
		if status != 200 {
			t.Fatalf("want 200, got %d body=%v", status, body)
		}
		data, _ := body["data"].(map[string]interface{})
		counts, ok := data["counts"].([]interface{})
		if !ok {
			t.Fatal("data.counts missing")
		}
		total, _ := data["total"].(float64)
		var sum float64
		for _, ci := range counts {
			c, _ := ci.(map[string]interface{})
			sum += c["count"].(float64)
		}
		if total != sum {
			t.Errorf("stray params caused aggregate drift: total=%v sum=%v", total, sum)
		}
	})

	t.Run("red_wrong_method_post_405_method_not_allowed", func(t *testing.T) {
		status, body, _ := doPost(t, base+"/api/reports/waste-summary", "{}")
		if status != 405 {
			t.Errorf("want 405, got %d", status)
		}
		code := getErrorCode(body)
		if code != "METHOD_NOT_ALLOWED" {
			t.Errorf("want METHOD_NOT_ALLOWED, got %q", code)
		}
		if _, hasErr := body["error"]; !hasErr {
			t.Error("error envelope missing for 405")
		}
	})
}

func TestReportsRedGreen_PaymentSummary(t *testing.T) {
	base := redgreenBaseURL(t)

	t.Run("green_canonical_shape_and_envelope", func(t *testing.T) {
		status, body, headers := doGet(t, base+"/api/reports/payment-summary")
		if status != 200 {
			t.Fatalf("want 200, got %d body=%v", status, body)
		}
		if xrid := headers.Get("X-Request-Id"); xrid == "" {
			t.Error("X-Request-Id header missing")
		}
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("body.data missing or not object: %v", body)
		}
		_, hasTotals := data["totals"]
		if !hasTotals {
			t.Error("data.totals missing")
		}
		_, hasRevenue := data["total_revenue"]
		if !hasRevenue {
			t.Error("data.total_revenue missing")
		}
	})

	t.Run("green_total_revenue_equals_sum_paid_invariant", func(t *testing.T) {
		_, body, _ := doGet(t, base+"/api/reports/payment-summary")
		data, _ := body["data"].(map[string]interface{})
		totals, _ := data["totals"].([]interface{})
		totalRevRaw, _ := data["total_revenue"].(string)

		var paidSum float64
		for _, ti := range totals {
			row, _ := ti.(map[string]interface{})
			sts, _ := row["status"].(string)
			amtRaw, _ := row["amount"].(string)
			amt, _ := parseFloatStr(amtRaw)
			if sts == "paid" {
				paidSum += amt
			}
		}
		totalRev, ok := parseFloatStr(totalRevRaw)
		if !ok {
			t.Fatalf("total_revenue not parseable: %q", totalRevRaw)
		}
		diff := totalRev - paidSum
		if diff < -0.005 || diff > 0.005 {
			t.Errorf("invariant: total_revenue=%v != sum(paid.amount)=%v", totalRev, paidSum)
		}
	})

	t.Run("green_amount_and_revenue_fixed_2dp", func(t *testing.T) {
		_, body, _ := doGet(t, base+"/api/reports/payment-summary")
		data, _ := body["data"].(map[string]interface{})
		totals, _ := data["totals"].([]interface{})
		totalRevRaw, _ := data["total_revenue"].(string)

		if !has2dp(totalRevRaw) {
			t.Errorf("total_revenue not 2dp string: %q", totalRevRaw)
		}
		for i, ti := range totals {
			row, _ := ti.(map[string]interface{})
			amtRaw, _ := row["amount"].(string)
			if !has2dp(amtRaw) {
				t.Errorf("totals[%d].amount not 2dp string: %q", i, amtRaw)
			}
		}
	})

	t.Run("green_enum_values_valid", func(t *testing.T) {
		_, body, _ := doGet(t, base+"/api/reports/payment-summary")
		data, _ := body["data"].(map[string]interface{})
		totals, _ := data["totals"].([]interface{})
		validStatuses := map[string]bool{"pending": true, "paid": true, "failed": true}
		for i, ti := range totals {
			row, _ := ti.(map[string]interface{})
			sts, _ := row["status"].(string)
			if !validStatuses[sts] {
				t.Errorf("totals[%d].status invalid: %q", i, sts)
			}
		}
	})

	t.Run("green_stray_query_params_ignored", func(t *testing.T) {
		status, body, _ := doGet(t, base+"/api/reports/payment-summary?limit=10&offset=5")
		if status != 200 {
			t.Fatalf("want 200, got %d body=%v", status, body)
		}
		data, _ := body["data"].(map[string]interface{})
		if _, ok := data["totals"]; !ok {
			t.Error("data.totals missing with stray params")
		}
	})

	t.Run("red_wrong_method_post_405_method_not_allowed", func(t *testing.T) {
		status, body, _ := doPost(t, base+"/api/reports/payment-summary", "{}")
		if status != 405 {
			t.Errorf("want 405, got %d", status)
		}
		code := getErrorCode(body)
		if code != "METHOD_NOT_ALLOWED" {
			t.Errorf("want METHOD_NOT_ALLOWED, got %q", code)
		}
	})
}

func TestReportsRedGreen_HouseholdHistory(t *testing.T) {
	base := redgreenBaseURL(t)

	t.Run("green_rich_history_all_statuses", func(t *testing.T) {
		hhID, doneID, cancelID, schedID := buildRichHistoryHousehold(t, base)

		status, body, headers := doGet(t, fmt.Sprintf("%s/api/reports/households/%s/history", base, hhID))
		if status != 200 {
			t.Fatalf("want 200, got %d body=%v", status, body)
		}
		if xrid := headers.Get("X-Request-Id"); xrid == "" {
			t.Error("X-Request-Id header missing")
		}

		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("body.data missing or not object: %v", body)
		}
		hh, ok := data["household"].(map[string]interface{})
		if !ok {
			t.Fatal("data.household missing")
		}
		if hh["id"] != hhID {
			t.Errorf("household.id: want %q, got %v", hhID, hh["id"])
		}

		pickups, ok := data["pickups"].([]interface{})
		if !ok {
			t.Fatal("data.pickups missing or not array")
		}
		if len(pickups) != 3 {
			t.Errorf("want 3 pickups, got %d", len(pickups))
		}

		statusByID := map[string]string{}
		for _, pi := range pickups {
			p, _ := pi.(map[string]interface{})
			id, _ := p["id"].(string)
			sts, _ := p["status"].(string)
			statusByID[id] = sts
		}
		if statusByID[doneID] != "completed" {
			t.Errorf("paper pickup: want completed, got %q", statusByID[doneID])
		}
		if statusByID[cancelID] != "canceled" {
			t.Errorf("plastic pickup: want canceled, got %q", statusByID[cancelID])
		}
		if statusByID[schedID] != "scheduled" {
			t.Errorf("electronic pickup: want scheduled, got %q", statusByID[schedID])
		}

		payments, ok := data["payments"].([]interface{})
		if !ok {
			t.Fatal("data.payments missing or not array")
		}
		if len(payments) != 1 {
			t.Errorf("want 1 payment (rule 5 auto-payment for completed paper), got %d", len(payments))
		} else {
			pay, _ := payments[0].(map[string]interface{})
			if pay["status"] != "pending" {
				t.Errorf("payment status: want pending, got %v", pay["status"])
			}
			if pay["amount"] != "10000.00" {
				t.Errorf("payment amount: want 10000.00, got %v", pay["amount"])
			}
			if pay["waste_id"] != doneID {
				t.Errorf("payment waste_id: want %q, got %v", doneID, pay["waste_id"])
			}
		}
	})

	t.Run("green_fresh_clean_household_empty_arrays_not_null", func(t *testing.T) {
		hhID := buildFreshCleanHousehold(t, base)

		status, body, _ := doGet(t, fmt.Sprintf("%s/api/reports/households/%s/history", base, hhID))
		if status != 200 {
			t.Fatalf("want 200, got %d body=%v", status, body)
		}
		data, ok := body["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("body.data missing or not object: %v", body)
		}
		pickups, ok := data["pickups"].([]interface{})
		if !ok {
			t.Fatal("data.pickups must be an array (not null) for clean household")
		}
		if len(pickups) != 0 {
			t.Errorf("want 0 pickups, got %d", len(pickups))
		}
		payments, ok := data["payments"].([]interface{})
		if !ok {
			t.Fatal("data.payments must be an array (not null) for clean household")
		}
		if len(payments) != 0 {
			t.Errorf("want 0 payments, got %d", len(payments))
		}
	})

	t.Run("red_non_uuid_id_400_validation_error_with_field", func(t *testing.T) {
		status, body, _ := doGet(t, base+"/api/reports/households/not-a-uuid/history")
		if status != 400 {
			t.Errorf("want 400, got %d", status)
		}
		code := getErrorCode(body)
		if code != "VALIDATION_ERROR" {
			t.Errorf("want VALIDATION_ERROR, got %q", code)
		}
		errObj, _ := body["error"].(map[string]interface{})
		details, _ := errObj["details"].([]interface{})
		if len(details) == 0 {
			t.Error("VALIDATION_ERROR must include details")
		} else {
			d, _ := details[0].(map[string]interface{})
			if d["field"] != "id" {
				t.Errorf("details[0].field: want 'id', got %v", d["field"])
			}
		}
	})

	t.Run("red_hex_dashes_invalid_uuid_400_validation_error", func(t *testing.T) {
		status, body, _ := doGet(t, base+"/api/reports/households/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx/history")
		if status != 400 {
			t.Errorf("want 400, got %d (must not be 404 or 500)", status)
		}
		code := getErrorCode(body)
		if code != "VALIDATION_ERROR" {
			t.Errorf("want VALIDATION_ERROR, got %q", code)
		}
	})

	t.Run("red_well_formed_unknown_uuid_404_household_not_found", func(t *testing.T) {
		status, body, _ := doGet(t, base+"/api/reports/households/00000000-0000-0000-0000-000000000099/history")
		if status != 404 {
			t.Errorf("want 404, got %d", status)
		}
		code := getErrorCode(body)
		if code != "HOUSEHOLD_NOT_FOUND" {
			t.Errorf("want HOUSEHOLD_NOT_FOUND, got %q", code)
		}
	})

	t.Run("red_wrong_method_delete_405_method_not_allowed", func(t *testing.T) {
		status, body, _ := doMethod(t, http.MethodDelete,
			base+"/api/reports/households/00000000-0000-0000-0000-000000000001/history", "")
		if status != 405 {
			t.Errorf("want 405, got %d", status)
		}
		code := getErrorCode(body)
		if code != "METHOD_NOT_ALLOWED" {
			t.Errorf("want METHOD_NOT_ALLOWED, got %q", code)
		}
	})
}

func TestReportsRedGreen_AggregationCorrectness(t *testing.T) {
	base := redgreenBaseURL(t)

	_, beforeWasteBody, _ := doGet(t, base+"/api/reports/waste-summary")
	beforeWasteData, _ := beforeWasteBody["data"].(map[string]interface{})
	beforeTotal, _ := beforeWasteData["total"].(float64)

	_, beforePayBody, _ := doGet(t, base+"/api/reports/payment-summary")
	beforePayData, _ := beforePayBody["data"].(map[string]interface{})
	beforeTotals, _ := beforePayData["totals"].([]interface{})
	var beforePendingCount float64
	for _, ti := range beforeTotals {
		row, _ := ti.(map[string]interface{})
		if row["status"] == "pending" {
			beforePendingCount, _ = row["count"].(float64)
		}
	}

	status, body, _ := doPost(t, base+"/api/households",
		`{"owner_name":"RG Agg Verify","address":"Jl. Verify 99"}`)
	if status != 201 {
		t.Fatalf("create household: want 201, got %d", status)
	}
	data, _ := body["data"].(map[string]interface{})
	hhID, _ := data["id"].(string)

	var pkID string
	for attempt := 0; attempt < 10; attempt++ {
		s, b, _ := doPost(t, base+"/api/pickups",
			fmt.Sprintf(`{"household_id":%q,"type":"organic"}`, hhID))
		if s == http.StatusTooManyRequests {
			time.Sleep(time.Second)
			continue
		}
		if s != 201 {
			t.Fatalf("create pickup: want 201, got %d body=%v", s, b)
		}
		d, _ := b["data"].(map[string]interface{})
		pkID, _ = d["id"].(string)
		break
	}
	if pkID == "" {
		t.Fatal("failed to create pickup after retries")
	}

	s, _, _ := doPut(t, fmt.Sprintf("%s/api/pickups/%s/schedule", base, pkID),
		`{"pickup_date":"2026-10-01T10:00:00Z"}`)
	if s != 200 {
		t.Fatalf("schedule: want 200, got %d", s)
	}

	s, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/complete", base, pkID), "")
	if s != 200 {
		t.Fatalf("complete: want 200, got %d", s)
	}

	_, afterWasteBody, _ := doGet(t, base+"/api/reports/waste-summary")
	afterWasteData, _ := afterWasteBody["data"].(map[string]interface{})
	afterTotal, _ := afterWasteData["total"].(float64)

	if afterTotal < beforeTotal+1 {
		t.Errorf("waste-summary total did not increase after completing pickup: before=%v after=%v", beforeTotal, afterTotal)
	}

	afterCounts, _ := afterWasteData["counts"].([]interface{})
	var afterSum float64
	for _, ci := range afterCounts {
		c, _ := ci.(map[string]interface{})
		afterSum += c["count"].(float64)
	}
	if afterTotal != afterSum {
		t.Errorf("waste-summary total!=sum(counts) after mutation: total=%v sum=%v", afterTotal, afterSum)
	}

	_, afterPayBody, _ := doGet(t, base+"/api/reports/payment-summary")
	afterPayData, _ := afterPayBody["data"].(map[string]interface{})
	afterTotals, _ := afterPayData["totals"].([]interface{})
	var afterPendingCount float64
	for _, ti := range afterTotals {
		row, _ := ti.(map[string]interface{})
		if row["status"] == "pending" {
			afterPendingCount, _ = row["count"].(float64)
		}
	}

	if afterPendingCount < beforePendingCount+1 {
		t.Errorf("payment-summary pending count did not increase: before=%v after=%v", beforePendingCount, afterPendingCount)
	}

	afterRevRaw, _ := afterPayData["total_revenue"].(string)
	afterRevFloat, ok := parseFloatStr(afterRevRaw)
	if !ok {
		t.Fatalf("total_revenue not parseable: %q", afterRevRaw)
	}
	var afterPaidSum float64
	for _, ti := range afterTotals {
		row, _ := ti.(map[string]interface{})
		if row["status"] == "paid" {
			amt, _ := row["amount"].(string)
			f, _ := parseFloatStr(amt)
			afterPaidSum += f
		}
	}
	diff := afterRevFloat - afterPaidSum
	if diff < -0.005 || diff > 0.005 {
		t.Errorf("payment invariant after mutation: total_revenue=%v != sum(paid)=%v", afterRevFloat, afterPaidSum)
	}
}

func TestReportsRedGreen_RouteNotFound(t *testing.T) {
	base := redgreenBaseURL(t)

	t.Run("red_nonexistent_report_route_404", func(t *testing.T) {
		status, body, _ := doGet(t, base+"/api/reports/nonexistent")
		if status != 404 {
			t.Errorf("want 404, got %d", status)
		}
		if errObj, ok := body["error"].(map[string]interface{}); ok {
			if errObj["code"] == "" {
				t.Error("error.code missing for unknown route")
			}
		} else {
			t.Error("error envelope missing for 404 unknown route")
		}
	})
}
