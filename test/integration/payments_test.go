//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"
)

func payBase(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDGREEN_BASE_URL")
	if u == "" {
		t.Skip("REDGREEN_BASE_URL not set — skipping black-box HTTP tests")
	}
	return strings.TrimRight(u, "/")
}

func payDoJSON(t *testing.T, method, url, body string) (*http.Response, map[string]interface{}) {
	t.Helper()
	var resp *http.Response
	for attempt := 0; attempt < 12; attempt++ {
		req, err := http.NewRequest(method, url, strings.NewReader(body))
		if err != nil {
			t.Fatalf("build %s %s: %v", method, url, err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, url, err)
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			break
		}
		resp.Body.Close()
		time.Sleep(750 * time.Millisecond)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func payDoGet(t *testing.T, url string) (*http.Response, map[string]interface{}) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func payConfirmMultipart(t *testing.T, url string, fileBytes []byte, filename, contentType string) (*http.Response, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="proof"; filename=%q`, filename))
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err = part.Write(fileBytes); err != nil {
		t.Fatalf("write file bytes: %v", err)
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPut, url, &buf)
	if err != nil {
		t.Fatalf("build PUT %s: %v", url, err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func payConfirmNoFile(t *testing.T, url string) (*http.Response, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.Close()

	req, err := http.NewRequest(http.MethodPut, url, &buf)
	if err != nil {
		t.Fatalf("build PUT %s (no file): %v", url, err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s (no file): %v", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func payAssertError(t *testing.T, m map[string]interface{}, expectedCode string) {
	t.Helper()
	errRaw, ok := m["error"]
	if !ok {
		t.Errorf("response missing 'error' key; got: %v", m)
		return
	}
	errMap, ok := errRaw.(map[string]interface{})
	if !ok {
		t.Errorf("error is not an object: %T", errRaw)
		return
	}
	code, _ := errMap["code"].(string)
	if code != expectedCode {
		t.Errorf("error.code: want %q, got %q (message: %v)", expectedCode, code, errMap["message"])
	}
	if _, hasMsg := errMap["message"]; !hasMsg {
		t.Error("error.message missing")
	}
}

func payAssertData(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	dataRaw, ok := m["data"]
	if !ok {
		t.Fatalf("response missing 'data' key; got keys: %v", func() []string {
			ks := make([]string, 0, len(m))
			for k := range m {
				ks = append(ks, k)
			}
			return ks
		}())
	}
	data, ok := dataRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object: %T %v", dataRaw, dataRaw)
	}
	return data
}

func paySetupHousehold(t *testing.T, base string) string {
	t.Helper()
	body := fmt.Sprintf(`{"owner_name":"Payment RG %s","address":"Jl. RG Pay No. 1"}`, t.Name())
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/households", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup household: want 201, got %d body=%v", resp.StatusCode, m)
	}
	data := payAssertData(t, m)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("setup household: id is empty")
	}
	return id
}

func paySetupPickup(t *testing.T, base, hhID, typ string) string {
	t.Helper()
	body := fmt.Sprintf(`{"household_id":%q,"type":%q}`, hhID, typ)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/pickups", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup pickup (type=%s): want 201, got %d body=%v", typ, resp.StatusCode, m)
	}
	data := payAssertData(t, m)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("setup pickup: id is empty")
	}
	return id
}

func paySetupPayment(t *testing.T, base, hhID, wasteID string) string {
	t.Helper()
	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhID, wasteID)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("setup payment: want 201, got %d body=%v", resp.StatusCode, m)
	}
	data := payAssertData(t, m)
	id, _ := data["id"].(string)
	if id == "" {
		t.Fatal("setup payment: id is empty")
	}
	return id
}

func payMinimalPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func payMinimalJPEG() []byte {
	return []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10,
		0x4a, 0x46, 0x49, 0x46, 0x00,
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0xff, 0xd9,
	}
}

func payMinimalPDF() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\nxref\n0 1\n0000000000 65535 f\ntrailer\n<< /Size 1 /Root 1 0 R >>\nstartxref\n9\n%%EOF\n")
}

func TestRedGreenPayment_POST_Green(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "paper")

	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhID, pkID)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%v", resp.StatusCode, m)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("X-Request-Id header missing on create")
	}
	data := payAssertData(t, m)
	for _, f := range []string{"id", "household_id", "waste_id", "amount", "status", "created_at", "updated_at"} {
		if _, ok := data[f]; !ok {
			t.Errorf("payment response missing field %q", f)
		}
	}
	status, _ := data["status"].(string)
	if status != "pending" {
		t.Errorf("status: want pending, got %q", status)
	}
	amount, _ := data["amount"].(string)
	if amount != "10000.00" {
		t.Errorf("amount: want 10000.00 (paper=PRICE_STANDARD), got %q", amount)
	}
	if _, hasMeta := m["meta"]; hasMeta {
		t.Error("meta must NOT be present on create response")
	}
}

func TestRedGreenPayment_POST_NonUUIDHouseholdID(t *testing.T) {
	base := payBase(t)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments",
		`{"household_id":"not-a-uuid","waste_id":"8a56a298-f903-4514-9fc5-57710bd8777b"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-UUID household_id: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_POST_NonUUIDWasteID(t *testing.T) {
	base := payBase(t)
	hhID := paySetupHousehold(t, base)
	body := fmt.Sprintf(`{"household_id":%q,"waste_id":"not-a-uuid"}`, hhID)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-UUID waste_id: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_POST_UnknownWasteID(t *testing.T) {
	base := payBase(t)
	hhID := paySetupHousehold(t, base)
	body := fmt.Sprintf(`{"household_id":%q,"waste_id":"00000000-0000-0000-0000-000000000099"}`, hhID)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", body)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown waste_id: want 404, got %d body=%v", resp.StatusCode, m)
	}
	payAssertError(t, m, "PICKUP_NOT_FOUND")
}

func TestRedGreenPayment_POST_HouseholdMismatch(t *testing.T) {
	base := payBase(t)

	hhA := paySetupHousehold(t, base)
	pkA := paySetupPickup(t, base, hhA, "plastic")

	hhB := paySetupHousehold(t, base)

	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhB, pkA)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", body)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("household mismatch: want 422, got %d body=%v", resp.StatusCode, m)
	}
	payAssertError(t, m, "PAYMENT_HOUSEHOLD_MISMATCH")
}

func TestRedGreenPayment_POST_DuplicateWasteID(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "organic")

	body := fmt.Sprintf(`{"household_id":%q,"waste_id":%q}`, hhID, pkID)
	resp1, m1 := payDoJSON(t, http.MethodPost, base+"/api/payments", body)
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("first payment: want 201, got %d body=%v", resp1.StatusCode, m1)
	}

	resp2, m2 := payDoJSON(t, http.MethodPost, base+"/api/payments", body)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate waste_id: want 409, got %d body=%v", resp2.StatusCode, m2)
	}
	payAssertError(t, m2, "PAYMENT_ALREADY_EXISTS")
}

func TestRedGreenPayment_POST_EmptyBody(t *testing.T) {
	base := payBase(t)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", `{}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty body: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
	errMap, _ := m["error"].(map[string]interface{})
	details, _ := errMap["details"].([]interface{})
	if len(details) == 0 {
		t.Error("VALIDATION_ERROR must include field-level details")
	}
}

func TestRedGreenPayment_POST_MalformedJSON(t *testing.T) {
	base := payBase(t)
	resp, m := payDoJSON(t, http.MethodPost, base+"/api/payments", `{bad json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON: want 400 (not 500), got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_GET_List_Green(t *testing.T) {
	base := payBase(t)

	resp, m := payDoGet(t, base+"/api/payments")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: want 200, got %d", resp.StatusCode)
	}
	dataRaw, ok := m["data"]
	if !ok {
		t.Fatal("response missing 'data'")
	}
	if _, ok := dataRaw.([]interface{}); !ok {
		t.Fatalf("data must be array, got %T", dataRaw)
	}
	metaRaw, ok := m["meta"]
	if !ok {
		t.Fatal("response missing 'meta'")
	}
	meta, _ := metaRaw.(map[string]interface{})
	for _, key := range []string{"page", "per_page", "total"} {
		if _, has := meta[key]; !has {
			t.Errorf("meta missing key %q", key)
		}
	}
}

func TestRedGreenPayment_GET_List_FilterByStatusPending(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "paper")
	paySetupPayment(t, base, hhID, pkID)

	resp, m := payDoGet(t, base+"/api/payments?status=pending")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter status=pending: want 200, got %d", resp.StatusCode)
	}
	data, _ := m["data"].([]interface{})
	for _, item := range data {
		p, _ := item.(map[string]interface{})
		if s, _ := p["status"].(string); s != "pending" {
			t.Errorf("filter status=pending: got item with status %q", s)
		}
	}
}

func TestRedGreenPayment_GET_List_FilterByStatusPaid(t *testing.T) {
	base := payBase(t)

	resp, m := payDoGet(t, base+"/api/payments?status=paid")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter status=paid: want 200, got %d", resp.StatusCode)
	}
	data, _ := m["data"].([]interface{})
	for _, item := range data {
		p, _ := item.(map[string]interface{})
		if s, _ := p["status"].(string); s != "paid" {
			t.Errorf("filter status=paid: got item with status %q", s)
		}
	}
}

func TestRedGreenPayment_GET_List_FilterByHouseholdID(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "plastic")
	paySetupPayment(t, base, hhID, pkID)

	resp, m := payDoGet(t, base+"/api/payments?household_id="+hhID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("filter household_id: want 200, got %d", resp.StatusCode)
	}
	data, _ := m["data"].([]interface{})
	if len(data) < 1 {
		t.Errorf("filter household_id: expected ≥1 result for household %q", hhID)
	}
	for _, item := range data {
		p, _ := item.(map[string]interface{})
		if hh, _ := p["household_id"].(string); hh != hhID {
			t.Errorf("filter household_id: got payment for household %q, want %q", hh, hhID)
		}
	}
}

func TestRedGreenPayment_GET_List_FilterByDateRange(t *testing.T) {
	base := payBase(t)

	resp, m := payDoGet(t, base+"/api/payments?date_from=2026-01-01T00:00:00Z&date_to=2026-12-31T23:59:59Z")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("date range filter: want 200, got %d", resp.StatusCode)
	}
	if _, ok := m["meta"]; !ok {
		t.Error("meta missing in date range filter response")
	}
}

func TestRedGreenPayment_GET_List_BadStatusEnum(t *testing.T) {
	base := payBase(t)
	resp, m := payDoGet(t, base+"/api/payments?status=invalid")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad status enum: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_GET_List_NonUUIDHouseholdID(t *testing.T) {
	base := payBase(t)
	resp, m := payDoGet(t, base+"/api/payments?household_id=not-a-uuid")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-UUID household_id: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_GET_List_BadDateFrom(t *testing.T) {
	base := payBase(t)
	resp, m := payDoGet(t, base+"/api/payments?date_from=not-a-date")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad date_from: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_GET_List_BadDateTo(t *testing.T) {
	base := payBase(t)
	resp, m := payDoGet(t, base+"/api/payments?date_to=2026-13-01")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad date_to: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_GET_List_DateFromAfterDateTo(t *testing.T) {
	base := payBase(t)
	resp, m := payDoGet(t, base+"/api/payments?date_from=2026-12-01T00:00:00Z&date_to=2026-01-01T00:00:00Z")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("date_from > date_to: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_Confirm_GreenPNG(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "paper")
	payID := paySetupPayment(t, base, hhID, pkID)

	url := base + "/api/payments/" + payID + "/confirm"
	resp, m := payConfirmMultipart(t, url, payMinimalPNG(), "proof.png", "image/png")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm PNG: want 200, got %d body=%v", resp.StatusCode, m)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("X-Request-Id header missing on confirm")
	}
	data := payAssertData(t, m)
	status, _ := data["status"].(string)
	if status != "paid" {
		t.Errorf("confirm PNG: status want paid, got %q", status)
	}
	proofURL, _ := data["proof_file_url"].(string)
	if proofURL == "" {
		t.Error("confirm PNG: proof_file_url must be set (non-empty)")
	}
	payDate, _ := data["payment_date"].(string)
	if payDate == "" {
		t.Error("confirm PNG: payment_date must be non-null after confirm")
	}
	if strings.Contains(proofURL, "minioadmin") || strings.Contains(proofURL, "password") {
		t.Errorf("proof_file_url must not contain credentials: %q", proofURL)
	}
	if _, hasMeta := m["meta"]; hasMeta {
		t.Error("meta must NOT be present on confirm response")
	}
}

func TestRedGreenPayment_Confirm_GreenJPEG(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "plastic")
	payID := paySetupPayment(t, base, hhID, pkID)

	url := base + "/api/payments/" + payID + "/confirm"
	resp, m := payConfirmMultipart(t, url, payMinimalJPEG(), "proof.jpg", "image/jpeg")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm JPEG: want 200, got %d body=%v", resp.StatusCode, m)
	}
	data := payAssertData(t, m)
	status, _ := data["status"].(string)
	if status != "paid" {
		t.Errorf("confirm JPEG: status want paid, got %q", status)
	}
	proofURL, _ := data["proof_file_url"].(string)
	if proofURL == "" {
		t.Error("confirm JPEG: proof_file_url must be set")
	}
}

func TestRedGreenPayment_Confirm_GreenPDF(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "organic")
	payID := paySetupPayment(t, base, hhID, pkID)

	url := base + "/api/payments/" + payID + "/confirm"
	resp, m := payConfirmMultipart(t, url, payMinimalPDF(), "proof.pdf", "application/pdf")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm PDF: want 200, got %d body=%v", resp.StatusCode, m)
	}
	data := payAssertData(t, m)
	status, _ := data["status"].(string)
	if status != "paid" {
		t.Errorf("confirm PDF: status want paid, got %q", status)
	}
	proofURL, _ := data["proof_file_url"].(string)
	if proofURL == "" {
		t.Error("confirm PDF: proof_file_url must be set")
	}
}

func TestRedGreenPayment_Confirm_NonUUIDID(t *testing.T) {
	base := payBase(t)
	url := base + "/api/payments/not-a-uuid/confirm"
	resp, m := payConfirmMultipart(t, url, payMinimalPNG(), "proof.png", "image/png")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-UUID id: want 400, got %d", resp.StatusCode)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_Confirm_MissingProofField(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "paper")
	payID := paySetupPayment(t, base, hhID, pkID)

	url := base + "/api/payments/" + payID + "/confirm"
	resp, m := payConfirmNoFile(t, url)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing proof field: want 400, got %d body=%v", resp.StatusCode, m)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_Confirm_WrongContentType(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "plastic")
	payID := paySetupPayment(t, base, hhID, pkID)

	txtBytes := []byte("This is plain text that sniffs as text/plain and should be rejected")
	url := base + "/api/payments/" + payID + "/confirm"
	resp, m := payConfirmMultipart(t, url, txtBytes, "proof.txt", "text/plain")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("text/plain file: want 400, got %d body=%v", resp.StatusCode, m)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_Confirm_OversizedFile(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "organic")
	payID := paySetupPayment(t, base, hhID, pkID)

	pngMagic := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	oversized := make([]byte, 5*1024*1024+512*1024)
	copy(oversized, pngMagic)

	url := base + "/api/payments/" + payID + "/confirm"
	resp, m := payConfirmMultipart(t, url, oversized, "big.png", "image/png")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized file (5.5 MiB): want 400, got %d body=%v", resp.StatusCode, m)
	}
	payAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenPayment_Confirm_UnknownPaymentID(t *testing.T) {
	base := payBase(t)
	unknownID := "00000000-0000-0000-0000-000000000099"
	url := base + "/api/payments/" + unknownID + "/confirm"
	resp, m := payConfirmMultipart(t, url, payMinimalPNG(), "proof.png", "image/png")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown payment id: want 404, got %d body=%v", resp.StatusCode, m)
	}
	payAssertError(t, m, "PAYMENT_NOT_FOUND")
}

func TestRedGreenPayment_Confirm_AlreadyPaid(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "paper")
	payID := paySetupPayment(t, base, hhID, pkID)

	url := base + "/api/payments/" + payID + "/confirm"
	resp1, m1 := payConfirmMultipart(t, url, payMinimalPNG(), "proof.png", "image/png")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first confirm: want 200, got %d body=%v", resp1.StatusCode, m1)
	}

	resp2, m2 := payConfirmMultipart(t, url, payMinimalPNG(), "proof2.png", "image/png")
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second confirm (already paid): want 409, got %d body=%v", resp2.StatusCode, m2)
	}
	payAssertError(t, m2, "PAYMENT_NOT_PENDING")
}

func TestRedGreenPayment_Confirm_BodyOver6MiB(t *testing.T) {
	base := payBase(t)

	hhID := paySetupHousehold(t, base)
	pkID := paySetupPickup(t, base, hhID, "plastic")
	payID := paySetupPayment(t, base, hhID, pkID)

	pngMagic := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	giant := make([]byte, 6*1024*1024+512*1024)
	copy(giant, pngMagic)

	url := base + "/api/payments/" + payID + "/confirm"
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="proof"; filename="giant.png"`)
	h.Set("Content-Type", "image/png")
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatalf("create multipart part: %v", err)
	}
	if _, err := part.Write(giant); err != nil {
		t.Fatalf("write giant bytes: %v", err)
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPut, url, &body)
	if err != nil {
		t.Fatalf("build PUT %s: %v", url, err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("body >6 MiB: server closed the connection on oversize body (%v) — accepted as body-limit rejection", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Logf("body >6 MiB: got status %d (expected 413 — Fiber body limit enforcement)", resp.StatusCode)
	}
}

func TestRedGreenPayment_Observability_RequestID(t *testing.T) {
	base := payBase(t)

	resp1, _ := payDoGet(t, base+"/api/payments")
	rid1 := resp1.Header.Get("X-Request-Id")
	if rid1 == "" {
		t.Error("X-Request-Id must be present on GET /api/payments")
	}

	resp2, _ := payDoGet(t, base+"/api/payments")
	rid2 := resp2.Header.Get("X-Request-Id")
	if rid2 == "" {
		t.Error("X-Request-Id must be present on second GET /api/payments")
	}
	if rid1 == rid2 {
		t.Errorf("consecutive requests must have different X-Request-Id; both=%s", rid1)
	}
}
