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

func redgreenBaseURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDGREEN_BASE_URL")
	if u == "" {
		t.Skip("REDGREEN_BASE_URL not set")
	}
	return strings.TrimRight(u, "/")
}

func doGet(t *testing.T, url string) (int, map[string]interface{}, http.Header) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var body map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Logf("non-JSON body: %s", raw)
		}
	}
	return resp.StatusCode, body, resp.Header
}

func doPost(t *testing.T, url string, jsonBody string) (int, map[string]interface{}, http.Header) {
	t.Helper()
	var resp *http.Response
	var err error
	for attempt := 0; attempt < 12; attempt++ {
		resp, err = http.Post(url, "application/json", strings.NewReader(jsonBody))
		if err != nil {
			t.Fatalf("POST %s: %v", url, err)
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			break
		}
		resp.Body.Close()
		time.Sleep(750 * time.Millisecond)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var body map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Logf("non-JSON body: %s", raw)
		}
	}
	return resp.StatusCode, body, resp.Header
}

func doPut(t *testing.T, url string, jsonBody string) (int, map[string]interface{}, http.Header) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPut, url, strings.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("new PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var body map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Logf("non-JSON body: %s", raw)
		}
	}
	return resp.StatusCode, body, resp.Header
}

func getErrorCode(body map[string]interface{}) string {
	if body == nil {
		return ""
	}
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		return ""
	}
	code, _ := errObj["code"].(string)
	return code
}

func getDataField(body map[string]interface{}, field string) interface{} {
	if body == nil {
		return nil
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	return data[field]
}

func buildIsolatedHousehold(t *testing.T, base string) (hhID string, pkID string) {
	t.Helper()

	status, body, _ := doPost(t, base+"/api/households",
		`{"owner_name":"RedGreen Report Tester","address":"Jl. Redgreen No. 1"}`)
	if status != 201 {
		t.Fatalf("create household: want 201, got %d", status)
	}
	data, _ := body["data"].(map[string]interface{})
	hhID, _ = data["id"].(string)
	if hhID == "" {
		t.Fatal("household id empty")
	}

	status, body, _ = doPost(t, base+"/api/pickups",
		fmt.Sprintf(`{"household_id":%q,"type":"organic"}`, hhID))
	if status != 201 {
		t.Fatalf("create pickup: want 201, got %d", status)
	}
	data, _ = body["data"].(map[string]interface{})
	pkID, _ = data["id"].(string)
	if pkID == "" {
		t.Fatal("pickup id empty")
	}

	status, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/schedule", base, pkID),
		`{"pickup_date":"2026-08-01T10:00:00Z"}`)
	if status != 200 {
		t.Fatalf("schedule pickup: want 200, got %d", status)
	}

	status, _, _ = doPut(t, fmt.Sprintf("%s/api/pickups/%s/complete", base, pkID), "")
	if status != 200 {
		t.Fatalf("complete pickup: want 200, got %d", status)
	}

	return hhID, pkID
}

func TestRedGreenReports_WasteSummary_Green(t *testing.T) {
	base := redgreenBaseURL(t)

	status, body, headers := doGet(t, base+"/api/reports/waste-summary")

	if status != 200 {
		t.Fatalf("want 200, got %d body=%v", status, body)
	}

	if xrid := headers.Get("X-Request-Id"); xrid == "" {
		t.Error("X-Request-Id header missing")
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing or non-object data key: %v", body)
	}

	counts, ok := data["counts"].([]interface{})
	if !ok {
		t.Fatal("data.counts missing or not array")
	}

	totalRaw, ok := data["total"]
	if !ok {
		t.Fatal("data.total missing")
	}
	total, ok := totalRaw.(float64)
	if !ok {
		t.Fatalf("data.total not numeric: %T %v", totalRaw, totalRaw)
	}

	validTypes := map[string]bool{"organic": true, "plastic": true, "paper": true, "electronic": true}
	validStatuses := map[string]bool{"pending": true, "scheduled": true, "completed": true, "canceled": true}

	var sumCounts float64
	for i, ci := range counts {
		c, ok := ci.(map[string]interface{})
		if !ok {
			t.Fatalf("counts[%d] not object", i)
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
		sumCounts += cnt
	}

	if total < 1 {
		t.Errorf("total should be >= 1, got %v", total)
	}
	if total != sumCounts {
		t.Errorf("invariant violation: total=%v != sum(counts)=%v", total, sumCounts)
	}
}

func TestRedGreenReports_PaymentSummary_Green(t *testing.T) {
	base := redgreenBaseURL(t)

	status, body, headers := doGet(t, base+"/api/reports/payment-summary")

	if status != 200 {
		t.Fatalf("want 200, got %d body=%v", status, body)
	}

	if xrid := headers.Get("X-Request-Id"); xrid == "" {
		t.Error("X-Request-Id header missing")
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing or non-object data key: %v", body)
	}

	totals, ok := data["totals"].([]interface{})
	if !ok {
		t.Fatal("data.totals missing or not array")
	}

	totalRevRaw, ok := data["total_revenue"]
	if !ok {
		t.Fatal("data.total_revenue missing")
	}
	totalRev, ok := totalRevRaw.(string)
	if !ok {
		t.Fatalf("total_revenue not string: %T %v", totalRevRaw, totalRevRaw)
	}

	validStatuses := map[string]bool{"pending": true, "paid": true, "failed": true}

	var paidAmountSum float64
	for i, ti := range totals {
		tRow, ok := ti.(map[string]interface{})
		if !ok {
			t.Fatalf("totals[%d] not object", i)
		}
		sts, _ := tRow["status"].(string)
		cnt, _ := tRow["count"].(float64)
		amtRaw, hasAmt := tRow["amount"]
		if !validStatuses[sts] {
			t.Errorf("totals[%d].status invalid: %q", i, sts)
		}
		if cnt < 0 {
			t.Errorf("totals[%d].count negative: %v", i, cnt)
		}
		if !hasAmt {
			t.Errorf("totals[%d].amount missing", i)
			continue
		}
		amtStr, ok := amtRaw.(string)
		if !ok {
			t.Errorf("totals[%d].amount not string: %T", i, amtRaw)
			continue
		}
		var amtFloat float64
		fmt.Sscanf(amtStr, "%f", &amtFloat)
		if amtFloat < 0 {
			t.Errorf("totals[%d].amount negative: %v", i, amtStr)
		}
		dotIdx := strings.Index(amtStr, ".")
		if dotIdx >= 0 {
			decimals := len(amtStr) - dotIdx - 1
			if decimals != 2 {
				t.Errorf("totals[%d].amount not fixed-2 decimal: %q", i, amtStr)
			}
		}
		if sts == "paid" {
			paidAmountSum += amtFloat
		}
	}

	dotIdx := strings.Index(totalRev, ".")
	if dotIdx >= 0 {
		decimals := len(totalRev) - dotIdx - 1
		if decimals != 2 {
			t.Errorf("total_revenue not fixed-2 decimal: %q", totalRev)
		}
	}

	var totalRevFloat float64
	fmt.Sscanf(totalRev, "%f", &totalRevFloat)

	diff := totalRevFloat - paidAmountSum
	if diff < -0.01 || diff > 0.01 {
		t.Errorf("invariant violation: total_revenue=%v != sum(paid amounts)=%v", totalRevFloat, paidAmountSum)
	}
}

func TestRedGreenReports_History_Green_IsolatedHousehold(t *testing.T) {
	base := redgreenBaseURL(t)

	hhID, pkID := buildIsolatedHousehold(t, base)

	status, body, headers := doGet(t, fmt.Sprintf("%s/api/reports/households/%s/history", base, hhID))
	if status != 200 {
		t.Fatalf("want 200, got %d body=%v", status, body)
	}

	if xrid := headers.Get("X-Request-Id"); xrid == "" {
		t.Error("X-Request-Id header missing")
	}

	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing or non-object data key: %v", body)
	}

	hh, ok := data["household"].(map[string]interface{})
	if !ok {
		t.Fatal("data.household missing")
	}
	if hh["id"] != hhID {
		t.Errorf("household.id mismatch: want %q, got %v", hhID, hh["id"])
	}

	pickups, ok := data["pickups"].([]interface{})
	if !ok {
		t.Fatal("data.pickups missing or not array")
	}
	if len(pickups) != 1 {
		t.Errorf("want 1 pickup, got %d", len(pickups))
	} else {
		pk, _ := pickups[0].(map[string]interface{})
		if pk["id"] != pkID {
			t.Errorf("pickup id mismatch: want %q, got %v", pkID, pk["id"])
		}
		if pk["status"] != "completed" {
			t.Errorf("pickup status: want 'completed', got %v", pk["status"])
		}
	}

	payments, ok := data["payments"].([]interface{})
	if !ok {
		t.Fatal("data.payments missing or not array")
	}
	if len(payments) != 1 {
		t.Errorf("want 1 payment (auto-created by rule 5), got %d", len(payments))
	} else {
		pay, _ := payments[0].(map[string]interface{})
		if pay["amount"] != "10000.00" {
			t.Errorf("payment amount: want '10000.00', got %v", pay["amount"])
		}
		if pay["status"] != "pending" {
			t.Errorf("payment status: want 'pending', got %v", pay["status"])
		}
		if pay["waste_id"] != pkID {
			t.Errorf("payment waste_id: want %q, got %v", pkID, pay["waste_id"])
		}
	}
}

func TestRedGreenReports_History_CrossRule_PaymentSummaryPending(t *testing.T) {
	base := redgreenBaseURL(t)

	hhID, _ := buildIsolatedHousehold(t, base)

	paymentsStatus, paymentsBody, _ := doGet(t,
		fmt.Sprintf("%s/api/payments?household_id=%s&status=pending", base, hhID))
	if paymentsStatus != 200 {
		t.Fatalf("GET payments: want 200, got %d", paymentsStatus)
	}
	payData, _ := paymentsBody["data"].([]interface{})
	if len(payData) == 0 {
		t.Fatal("no pending payments found for isolated household; rule-5 auto-payment missing")
	}
	pay, _ := payData[0].(map[string]interface{})
	if pay["amount"] != "10000.00" {
		t.Errorf("auto-payment amount: want '10000.00', got %v", pay["amount"])
	}

	summaryStatus, summaryBody, _ := doGet(t, base+"/api/reports/payment-summary")
	if summaryStatus != 200 {
		t.Fatalf("GET payment-summary: want 200, got %d", summaryStatus)
	}
	summaryData, _ := summaryBody["data"].(map[string]interface{})
	totals, _ := summaryData["totals"].([]interface{})
	var pendingCount float64
	for _, ti := range totals {
		row, _ := ti.(map[string]interface{})
		if row["status"] == "pending" {
			pendingCount, _ = row["count"].(float64)
		}
	}
	if pendingCount < 1 {
		t.Errorf("payment-summary pending count should be >= 1 (rule-5 payment exists), got %v", pendingCount)
	}
}

func TestRedGreenReports_History_Red_NonUUIDId(t *testing.T) {
	base := redgreenBaseURL(t)

	status, body, _ := doGet(t, base+"/api/reports/households/not-a-uuid/history")
	if status != 400 {
		t.Errorf("want 400 for non-uuid id, got %d", status)
	}
	code := getErrorCode(body)
	if code != "VALIDATION_ERROR" {
		t.Errorf("want error.code=VALIDATION_ERROR, got %q", code)
	}
}

func TestRedGreenReports_History_Red_UnknownUUID(t *testing.T) {
	base := redgreenBaseURL(t)

	status, body, _ := doGet(t, base+"/api/reports/households/00000000-0000-0000-0000-000000000099/history")
	if status != 404 {
		t.Errorf("want 404 for unknown uuid, got %d", status)
	}
	code := getErrorCode(body)
	if code != "HOUSEHOLD_NOT_FOUND" {
		t.Errorf("want error.code=HOUSEHOLD_NOT_FOUND, got %q", code)
	}
}

func TestRedGreenReports_WasteSummary_Red_WrongMethod(t *testing.T) {
	base := redgreenBaseURL(t)

	status, body, _ := doPost(t, base+"/api/reports/waste-summary", "{}")
	if status != 405 {
		t.Errorf("want 405 for POST /api/reports/waste-summary, got %d", status)
	}
	code := getErrorCode(body)
	if code != "METHOD_NOT_ALLOWED" {
		t.Logf("note: error.code=%q (may be framework default)", code)
	}
}

func TestRedGreenReports_Reports_Red_RouteNotFound(t *testing.T) {
	base := redgreenBaseURL(t)

	status, body, _ := doGet(t, base+"/api/reports/nonexistent")
	if status != 404 {
		t.Errorf("want 404 for /api/reports/nonexistent, got %d", status)
	}
	code := getErrorCode(body)
	if code != "NOT_FOUND" {
		t.Errorf("want error.code=NOT_FOUND, got %q", code)
	}
}
