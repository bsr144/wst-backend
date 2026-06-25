//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"
)

func rgPmBase(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDGREEN_BASE_URL")
	if u == "" {
		t.Skip("REDGREEN_BASE_URL not set — skipping payments red-green tests")
	}
	return strings.TrimRight(u, "/")
}

func rgPmRequest(t *testing.T, method, url, contentType, body string) (*http.Response, map[string]any) {
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

func rgPmAssertErrorCode(t *testing.T, m map[string]any, want string) {
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

func rgPmCreateHousehold(t *testing.T, base, name string) string {
	t.Helper()
	body := fmt.Sprintf(`{"owner_name":%q,"address":"Jl. Bayar 1"}`, name)
	resp, m := rgPmRequest(t, http.MethodPost, base+"/api/households", "application/json", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("rgPmCreateHousehold: want 201, got %d body=%v", resp.StatusCode, m)
	}
	data, _ := m["data"].(map[string]any)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("rgPmCreateHousehold: data.id empty")
	}
	return id
}

func rgPmCreatePickup(t *testing.T, base, hhID, typ string, safetyCheck *bool) string {
	t.Helper()
	body := fmt.Sprintf(`{"household_id":%q,"type":%q`, hhID, typ)
	if safetyCheck != nil {
		if *safetyCheck {
			body += `,"safety_check":true`
		} else {
			body += `,"safety_check":false`
		}
	}
	body += `}`
	for i := 0; i < 6; i++ {
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/pickups", "application/json", body)
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("rgPmCreatePickup: want 201, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatal("rgPmCreatePickup: data.id empty")
		}
		return id
	}
	t.Fatal("rgPmCreatePickup: exhausted retries on rate limit")
	return ""
}

func rgPmSchedulePickup(t *testing.T, base, pickupID string) {
	t.Helper()
	resp, m := rgPmRequest(t, http.MethodPut,
		base+"/api/pickups/"+pickupID+"/schedule",
		"application/json",
		`{"pickup_date":"2026-07-01T09:00:00Z"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rgPmSchedulePickup: want 200, got %d body=%v", resp.StatusCode, m)
	}
}

func rgPmCompletePickup(t *testing.T, base, pickupID string) {
	t.Helper()
	resp, m := rgPmRequest(t, http.MethodPut, base+"/api/pickups/"+pickupID+"/complete", "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rgPmCompletePickup: want 200, got %d body=%v", resp.StatusCode, m)
	}
}

func rgPmGetPaymentForWaste(t *testing.T, base, hhID, wasteID string) string {
	t.Helper()
	resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?household_id="+hhID, "", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rgPmGetPaymentForWaste: want 200, got %d", resp.StatusCode)
	}
	dataRaw, _ := m["data"].([]any)
	for _, item := range dataRaw {
		p, _ := item.(map[string]any)
		if wid, _ := p["waste_id"].(string); wid == wasteID {
			id, _ := p["id"].(string)
			return id
		}
	}
	t.Fatalf("rgPmGetPaymentForWaste: no payment found for waste_id=%s in household=%s", wasteID, hhID)
	return ""
}

func rgPmMinimalPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func rgPmConfirmRequest(t *testing.T, base, paymentID string, filename string, fileContentType string, fileData []byte) (*http.Response, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="proof"; filename=%q`, filename))
	h.Set("Content-Type", fileContentType)
	fw, err := w.CreatePart(h)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := fw.Write(fileData); err != nil {
		t.Fatalf("write part: %v", err)
	}
	w.Close()

	req, err := http.NewRequest(http.MethodPut,
		base+"/api/payments/"+paymentID+"/confirm",
		&body)
	if err != nil {
		t.Fatalf("build confirm request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("confirm %s: %v", paymentID, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return resp, m
}

func TestPaymentsRedGreen(t *testing.T) {
	base := rgPmBase(t)

	t.Run("G01_GET_list_200_with_meta", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if _, ok := m["data"].([]any); !ok {
			t.Fatalf("data must be array, got %T", m["data"])
		}
		meta, ok := m["meta"].(map[string]any)
		if !ok {
			t.Fatal("meta missing or not an object")
		}
		for _, key := range []string{"page", "per_page", "total"} {
			if _, has := meta[key]; !has {
				t.Errorf("meta missing key %q", key)
			}
		}
		if resp.Header.Get("X-Request-Id") == "" {
			t.Error("X-Request-Id header missing")
		}
	})

	t.Run("G02_GET_filter_status_paid_200", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?status=paid", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if _, ok := m["meta"]; !ok {
			t.Error("meta missing on status-filtered list")
		}
		items, _ := m["data"].([]any)
		for _, item := range items {
			p, _ := item.(map[string]any)
			if s, _ := p["status"].(string); s != "paid" {
				t.Errorf("filtered list: got status=%q, want paid", s)
			}
		}
	})

	t.Run("G03_GET_filter_status_pending_200", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?status=pending", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		items, _ := m["data"].([]any)
		for _, item := range items {
			p, _ := item.(map[string]any)
			if s, _ := p["status"].(string); s != "pending" {
				t.Errorf("filtered list: got status=%q, want pending", s)
			}
		}
	})

	t.Run("G04_GET_filter_household_id_200", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG4 Pay FilterHH")
		trueVal := true
		pkID := rgPmCreatePickup(t, base, hhID, "electronic", &trueVal)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)

		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?household_id="+hhID, "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		items, _ := m["data"].([]any)
		if len(items) == 0 {
			t.Fatal("expected at least one payment for the household")
		}
		for _, item := range items {
			p, _ := item.(map[string]any)
			if gotHH, _ := p["household_id"].(string); gotHH != hhID {
				t.Errorf("got household_id=%q, want %q", gotHH, hhID)
			}
		}
	})

	t.Run("G05_GET_filter_date_range_RFC3339_200", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG5 Pay DateRange")
		trueVal := true
		pkID := rgPmCreatePickup(t, base, hhID, "electronic", &trueVal)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)
		pmID := rgPmGetPaymentForWaste(t, base, hhID, pkID)
		rgPmConfirmRequest(t, base, pmID, "proof.png", "image/png", rgPmMinimalPNG())

		now := time.Now().UTC()
		from := now.Add(-1 * time.Hour).Format(time.RFC3339)
		to := now.Add(1 * time.Hour).Format(time.RFC3339)
		url := fmt.Sprintf("%s/api/payments?date_from=%s&date_to=%s", base, from, to)
		resp, m := rgPmRequest(t, http.MethodGet, url, "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if _, ok := m["meta"]; !ok {
			t.Error("meta missing on date-range-filtered list")
		}
	})

	t.Run("G06_POST_manual_create_pending_pickup_201", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG6 Pay ManualCreate")
		pkID := rgPmCreatePickup(t, base, hhID, "plastic", nil)

		body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhID, pkID)
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("manual create for pending pickup: want 201, got %d body=%v", resp.StatusCode, m)
		}
		data, _ := m["data"].(map[string]any)
		if id, _ := data["id"].(string); id == "" {
			t.Error("data.id missing from created payment")
		}
		if status, _ := data["status"].(string); status != "pending" {
			t.Errorf("data.status: want pending, got %q", status)
		}
		if amount, _ := data["amount"].(string); amount == "" {
			t.Error("data.amount must be set (auto-calculated from pickup type)")
		}
	})

	t.Run("G07_PUT_confirm_valid_PNG_rule6_200_paid_proof_url_set", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG7 Pay Confirm")
		trueVal := true
		pkID := rgPmCreatePickup(t, base, hhID, "electronic", &trueVal)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)
		pmID := rgPmGetPaymentForWaste(t, base, hhID, pkID)

		resp, m := rgPmConfirmRequest(t, base, pmID, "proof.png", "image/png", rgPmMinimalPNG())
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("confirm: want 200, got %d body=%v", resp.StatusCode, m)
		}
		if resp.Header.Get("X-Request-Id") == "" {
			t.Error("X-Request-Id missing on confirm response")
		}
		data, _ := m["data"].(map[string]any)
		if status, _ := data["status"].(string); status != "paid" {
			t.Errorf("status after confirm: want paid, got %q", status)
		}
		proofURL, _ := data["proof_file_url"].(string)
		if proofURL == "" {
			t.Error("proof_file_url must be non-empty after confirm (rule 6)")
		}
		if data["payment_date"] == nil {
			t.Error("payment_date must be set after confirm")
		}
		if _, hasMeta := m["meta"]; hasMeta {
			t.Error("meta must NOT be present on confirm response")
		}
	})

	t.Run("G08_POST_duplicate_payment_409_PAYMENT_ALREADY_EXISTS", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG8 Pay Dup")
		trueVal := true
		pkID := rgPmCreatePickup(t, base, hhID, "electronic", &trueVal)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)

		body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhID, pkID)
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json", body)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("duplicate payment: want 409, got %d body=%v", resp.StatusCode, m)
		}
		rgPmAssertErrorCode(t, m, "PAYMENT_ALREADY_EXISTS")
	})

	t.Run("R01_POST_missing_household_id_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json",
			`{"waste_id":"00000000-0000-0000-0000-000000000001"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "household_id" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=household_id")
		}
	})

	t.Run("R02_POST_missing_waste_id_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json",
			`{"household_id":"00000000-0000-0000-0000-000000000001"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "waste_id" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=waste_id")
		}
	})

	t.Run("R03_POST_non_uuid_household_id_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json",
			`{"household_id":"not-a-uuid","waste_id":"00000000-0000-0000-0000-000000000001"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R04_POST_non_uuid_waste_id_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json",
			`{"household_id":"00000000-0000-0000-0000-000000000001","waste_id":"not-a-uuid"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R05_POST_malformed_json_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json",
			`{"household_id":`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("malformed JSON: want 400 (not 500), got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R06_POST_unknown_waste_uuid_404_PICKUP_NOT_FOUND", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG R06 Pay")
		body := fmt.Sprintf(`{"household_id":%q,"waste_id":"00000000-0000-0000-0000-000000000099"}`, hhID)
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json", body)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("unknown waste uuid: want 404, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "PICKUP_NOT_FOUND")
	})

	t.Run("R07_POST_household_mismatch_422_PAYMENT_HOUSEHOLD_MISMATCH", func(t *testing.T) {
		hhA := rgPmCreateHousehold(t, base, "RG R07A Pay")
		hhB := rgPmCreateHousehold(t, base, "RG R07B Pay")
		pkID := rgPmCreatePickup(t, base, hhB, "plastic", nil)

		body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhA, pkID)
		resp, m := rgPmRequest(t, http.MethodPost, base+"/api/payments", "application/json", body)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("household mismatch: want 422, got %d body=%v", resp.StatusCode, m)
		}
		rgPmAssertErrorCode(t, m, "PAYMENT_HOUSEHOLD_MISMATCH")
	})

	t.Run("R08_GET_bad_status_enum_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?status=invalid", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("bad status: want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "status" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=status")
		}
	})

	t.Run("R09_GET_bad_household_id_non_uuid_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?household_id=not-a-uuid", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("non-UUID household_id: want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R10_GET_bad_date_from_not_RFC3339_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet, base+"/api/payments?date_from=2026-06-25", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("bad date_from: want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "date_from" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=date_from")
		}
	})

	t.Run("R11_GET_date_from_after_date_to_400", func(t *testing.T) {
		resp, m := rgPmRequest(t, http.MethodGet,
			base+"/api/payments?date_from=2026-06-25T23:00:00Z&date_to=2026-06-25T00:00:00Z", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("date_from>date_to: want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R12_PUT_confirm_no_file_400_field_proof", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG R12 Confirm NoFile")
		pkID := rgPmCreatePickup(t, base, hhID, "plastic", nil)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)
		pmID := rgPmGetPaymentForWaste(t, base, hhID, pkID)

		req, _ := http.NewRequest(http.MethodPut, base+"/api/payments/"+pmID+"/confirm", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("confirm no file: %v", err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("no file: want 400, got %d", resp.StatusCode)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "proof" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=proof")
		}
	})

	t.Run("R13_PUT_confirm_wrong_content_type_text_plain_400", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG R13 Confirm WrongType")
		pkID := rgPmCreatePickup(t, base, hhID, "plastic", nil)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)
		pmID := rgPmGetPaymentForWaste(t, base, hhID, pkID)

		textData := []byte("This is plain text, not a PNG or PDF. AAABBBCCC")
		resp, m := rgPmConfirmRequest(t, base, pmID, "proof.txt", "text/plain", textData)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("wrong content-type: want 400, got %d body=%v", resp.StatusCode, m)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "proof" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=proof for unsupported content type")
		}
	})

	t.Run("R14_PUT_confirm_oversized_file_400_file_too_large", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG R14 Confirm Oversize")
		pkID := rgPmCreatePickup(t, base, hhID, "plastic", nil)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)
		pmID := rgPmGetPaymentForWaste(t, base, hhID, pkID)

		pngHeader := []byte("\x89PNG\r\n\x1a\n")
		padding := bytes.Repeat([]byte{0x00}, 5*1024*1024+512)
		oversized := append(pngHeader, padding...)

		resp, m := rgPmConfirmRequest(t, base, pmID, "big.png", "image/png", oversized)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("oversized file: want 400, got %d body=%v", resp.StatusCode, m)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R15_PUT_confirm_already_paid_409_PAYMENT_NOT_PENDING", func(t *testing.T) {
		hhID := rgPmCreateHousehold(t, base, "RG R15 Confirm AlreadyPaid")
		trueVal := true
		pkID := rgPmCreatePickup(t, base, hhID, "electronic", &trueVal)
		rgPmSchedulePickup(t, base, pkID)
		rgPmCompletePickup(t, base, pkID)
		pmID := rgPmGetPaymentForWaste(t, base, hhID, pkID)

		resp1, _ := rgPmConfirmRequest(t, base, pmID, "proof.png", "image/png", rgPmMinimalPNG())
		if resp1.StatusCode != http.StatusOK {
			t.Fatalf("first confirm: want 200, got %d", resp1.StatusCode)
		}

		resp2, m2 := rgPmConfirmRequest(t, base, pmID, "proof.png", "image/png", rgPmMinimalPNG())
		if resp2.StatusCode != http.StatusConflict {
			t.Fatalf("second confirm on paid: want 409, got %d body=%v", resp2.StatusCode, m2)
		}
		rgPmAssertErrorCode(t, m2, "PAYMENT_NOT_PENDING")
	})

	t.Run("R16_PUT_confirm_unknown_uuid_404_PAYMENT_NOT_FOUND", func(t *testing.T) {
		resp, m := rgPmConfirmRequest(t, base,
			"00000000-0000-0000-0000-000000000099",
			"proof.png", "image/png", rgPmMinimalPNG())
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("unknown payment UUID: want 404, got %d body=%v", resp.StatusCode, m)
		}
		rgPmAssertErrorCode(t, m, "PAYMENT_NOT_FOUND")
	})

	t.Run("R17_PUT_confirm_non_uuid_path_400_VALIDATION_ERROR", func(t *testing.T) {
		resp, m := rgPmConfirmRequest(t, base, "not-a-uuid", "proof.png", "image/png", rgPmMinimalPNG())
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("non-UUID path: want 400, got %d body=%v", resp.StatusCode, m)
		}
		rgPmAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "id" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=id for non-UUID path param")
		}
	})

	t.Run("OBS01_X_Request_Id_unique_per_request", func(t *testing.T) {
		resp1, _ := rgPmRequest(t, http.MethodGet, base+"/api/payments", "", "")
		rid1 := resp1.Header.Get("X-Request-Id")
		if rid1 == "" {
			t.Error("X-Request-Id must be present on first request")
		}
		resp2, _ := rgPmRequest(t, http.MethodGet, base+"/api/payments", "", "")
		rid2 := resp2.Header.Get("X-Request-Id")
		if rid2 == "" {
			t.Error("X-Request-Id must be present on second request")
		}
		if rid1 == rid2 {
			t.Errorf("consecutive requests must have unique X-Request-Id; both=%q", rid1)
		}
	})
}
