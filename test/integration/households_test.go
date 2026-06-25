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
)

func hhRedgreenBase(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDGREEN_BASE_URL")
	if u == "" {
		t.Skip("REDGREEN_BASE_URL not set — skipping black-box HTTP tests")
	}
	return strings.TrimRight(u, "/")
}

func hhDoPost(t *testing.T, base string, body string) (*http.Response, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/api/households", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build POST /api/households: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/households: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func hhDoGet(t *testing.T, base, path string) (*http.Response, map[string]interface{}) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func hhDoDelete(t *testing.T, base, id string) (*http.Response, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, base+"/api/households/"+id, nil)
	if err != nil {
		t.Fatalf("build DELETE: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /api/households/%s: %v", id, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	if len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return resp, m
}

func hhDoMethod(t *testing.T, method, base, path, body string) (*http.Response, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return resp, m
}

func hhAssertData(t *testing.T, m map[string]interface{}) map[string]interface{} {
	t.Helper()
	dataRaw, ok := m["data"]
	if !ok {
		t.Errorf("response missing 'data' key; got keys: %v", hhMapKeys(m))
		return nil
	}
	data, ok := dataRaw.(map[string]interface{})
	if !ok {
		t.Errorf("data must be an object, got %T", dataRaw)
		return nil
	}
	return data
}

func hhAssertError(t *testing.T, m map[string]interface{}, expectedCode string) {
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
		t.Errorf("error.code: want %q, got %q", expectedCode, code)
	}
	if _, hasMsg := errMap["message"]; !hasMsg {
		t.Errorf("error.message missing")
	}
}

func hhAssertFields(t *testing.T, data map[string]interface{}) string {
	t.Helper()
	for _, f := range []string{"id", "owner_name", "address", "created_at", "updated_at"} {
		if _, ok := data[f]; !ok {
			t.Errorf("household data missing field %q", f)
		}
	}
	id, _ := data["id"].(string)
	return id
}

func hhMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func hhCreate(t *testing.T, base, name, address string) string {
	t.Helper()
	body := fmt.Sprintf(`{"owner_name":%q,"address":%q}`, name, address)
	resp, m := hhDoPost(t, base, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("hhCreate: want 201, got %d body=%v", resp.StatusCode, m)
	}
	data := hhAssertData(t, m)
	return hhAssertFields(t, data)
}

func TestRedGreenHousehold_POST_Green(t *testing.T) {
	base := hhRedgreenBase(t)

	resp, m := hhDoPost(t, base, `{"owner_name":"RG Test Owner","address":"Jl. RG No. 1, Jakarta"}`)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%v", resp.StatusCode, m)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("X-Request-Id header missing")
	}
	data := hhAssertData(t, m)
	id := hhAssertFields(t, data)
	if id == "" {
		t.Error("data.id is empty")
	}
	if _, hasMeta := m["meta"]; hasMeta {
		t.Error("meta must NOT be present on non-list response")
	}

	_, _ = hhDoDelete(t, base, id)
}

func TestRedGreenHousehold_POST_EmptyBody(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoPost(t, base, `{}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
	errMap, _ := m["error"].(map[string]interface{})
	details, _ := errMap["details"].([]interface{})
	if len(details) == 0 {
		t.Error("VALIDATION_ERROR must include field-level details")
	}
}

func TestRedGreenHousehold_POST_MissingOwnerName(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoPost(t, base, `{"address":"Jl. Test"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
	errMap, _ := m["error"].(map[string]interface{})
	details, _ := errMap["details"].([]interface{})
	found := false
	for _, d := range details {
		dm, _ := d.(map[string]interface{})
		if dm["field"] == "owner_name" {
			found = true
		}
	}
	if !found {
		t.Error("details must include field=owner_name")
	}
}

func TestRedGreenHousehold_POST_MissingAddress(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoPost(t, base, `{"owner_name":"Test"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
	errMap, _ := m["error"].(map[string]interface{})
	details, _ := errMap["details"].([]interface{})
	found := false
	for _, d := range details {
		dm, _ := d.(map[string]interface{})
		if dm["field"] == "address" {
			found = true
		}
	}
	if !found {
		t.Error("details must include field=address")
	}
}

func TestRedGreenHousehold_POST_OwnerNameTooLong(t *testing.T) {
	base := hhRedgreenBase(t)
	long := strings.Repeat("A", 256)
	body := fmt.Sprintf(`{"owner_name":%q,"address":"Jl. Test"}`, long)
	resp, m := hhDoPost(t, base, body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("owner_name=256 chars: want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenHousehold_POST_AddressTooLong(t *testing.T) {
	base := hhRedgreenBase(t)
	long := strings.Repeat("B", 1001)
	body := fmt.Sprintf(`{"owner_name":"Test","address":%q}`, long)
	resp, m := hhDoPost(t, base, body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("address=1001 chars: want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenHousehold_POST_WhitespaceOnlyOwnerName(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoPost(t, base, `{"owner_name":"   ","address":"Jl. Test"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("whitespace-only owner_name: want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenHousehold_POST_MalformedJSON(t *testing.T) {
	base := hhRedgreenBase(t)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/households", strings.NewReader(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed JSON: want 400 (not 500), got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenHousehold_POST_WrongContentType(t *testing.T) {
	base := hhRedgreenBase(t)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/households",
		strings.NewReader(`{"owner_name":"Test","address":"Jl."}`))
	req.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong Content-Type: want 400, got %d", resp.StatusCode)
	}
}

func TestRedGreenHousehold_POST_BoundaryValid(t *testing.T) {
	base := hhRedgreenBase(t)

	name255 := strings.Repeat("C", 255)
	body := fmt.Sprintf(`{"owner_name":%q,"address":"Jl. Boundary"}`, name255)
	resp, m := hhDoPost(t, base, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("owner_name=255: want 201, got %d", resp.StatusCode)
	}
	data := hhAssertData(t, m)
	id := hhAssertFields(t, data)
	_, _ = hhDoDelete(t, base, id)

	addr1000 := strings.Repeat("D", 1000)
	body = fmt.Sprintf(`{"owner_name":"Boundary","address":%q}`, addr1000)
	resp, m = hhDoPost(t, base, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("address=1000: want 201, got %d", resp.StatusCode)
	}
	data = hhAssertData(t, m)
	id = hhAssertFields(t, data)
	_, _ = hhDoDelete(t, base, id)
}

func TestRedGreenHousehold_GET_List_Green(t *testing.T) {
	base := hhRedgreenBase(t)
	id := hhCreate(t, base, "List Test RG", "Jl. List RG No. 1")
	defer func() { _, _ = hhDoDelete(t, base, id) }()

	resp, m := hhDoGet(t, base, "/api/households")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
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
	meta, ok := metaRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("meta must be object, got %T", metaRaw)
	}
	for _, key := range []string{"page", "per_page", "total"} {
		if _, has := meta[key]; !has {
			t.Errorf("meta missing key %q", key)
		}
	}
}

func TestRedGreenHousehold_GET_List_PerPageCap(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoGet(t, base, "/api/households?per_page=200")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	meta, _ := m["meta"].(map[string]interface{})
	perPage, _ := meta["per_page"].(float64)
	if perPage > 100 {
		t.Errorf("per_page should be capped to 100, got %v", perPage)
	}
}

func TestRedGreenHousehold_GET_List_InvalidParamsSilentDefault(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoGet(t, base, "/api/households?page=abc&per_page=abc")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("invalid params must silently default (200), got %d", resp.StatusCode)
	}
	if _, hasMeta := m["meta"]; !hasMeta {
		t.Error("meta missing")
	}
}

func TestRedGreenHousehold_GET_ByID_Green(t *testing.T) {
	base := hhRedgreenBase(t)
	id := hhCreate(t, base, "GetByID RG", "Jl. GetByID RG No. 1")
	defer func() { _, _ = hhDoDelete(t, base, id) }()

	resp, m := hhDoGet(t, base, "/api/households/"+id)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d body=%v", resp.StatusCode, m)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("X-Request-Id header missing")
	}
	data := hhAssertData(t, m)
	gotID := hhAssertFields(t, data)
	if gotID != id {
		t.Errorf("data.id: want %q, got %q", id, gotID)
	}
	if _, hasMeta := m["meta"]; hasMeta {
		t.Error("meta must NOT be present on single-resource response")
	}
}

func TestRedGreenHousehold_GET_ByID_NonUUID(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoGet(t, base, "/api/households/not-a-uuid")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-UUID id: want 400 (not 404/500), got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenHousehold_GET_ByID_UnknownUUID(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoGet(t, base, "/api/households/00000000-0000-0000-0000-000000000001")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown UUID: want 404, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "HOUSEHOLD_NOT_FOUND")
}

func TestRedGreenHousehold_DELETE_Green(t *testing.T) {
	base := hhRedgreenBase(t)
	id := hhCreate(t, base, "Delete Me RG", "Jl. Delete RG No. 1")

	resp, m := hhDoDelete(t, base, id)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%v", resp.StatusCode, m)
	}
	if len(m) != 0 {
		t.Errorf("204 body must be empty; got %v", m)
	}

	resp2, m2 := hhDoDelete(t, base, id)
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete: want 404, got %d body=%v", resp2.StatusCode, m2)
	}
	hhAssertError(t, m2, "HOUSEHOLD_NOT_FOUND")
}

func TestRedGreenHousehold_DELETE_NonUUID(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoDelete(t, base, "not-a-uuid")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-UUID: want 400, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "VALIDATION_ERROR")
}

func TestRedGreenHousehold_DELETE_UnknownUUID(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoDelete(t, base, "00000000-0000-0000-0000-000000000002")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown UUID: want 404, got %d", resp.StatusCode)
	}
	hhAssertError(t, m, "HOUSEHOLD_NOT_FOUND")
}

func TestRedGreenHousehold_DELETE_WithDependents(t *testing.T) {
	base := hhRedgreenBase(t)

	hhID := hhCreate(t, base, "Dependent RG", "Jl. Dependent RG No. 1")

	body := fmt.Sprintf(`{"household_id":%q,"type":"organic"}`, hhID)
	req, _ := http.NewRequest(http.MethodPost, base+"/api/pickups", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	pResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create pickup: %v", err)
	}
	defer pResp.Body.Close()
	if pResp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(pResp.Body)
		t.Fatalf("create pickup: want 201, got %d body=%s", pResp.StatusCode, raw)
	}

	resp, m := hhDoDelete(t, base, hhID)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("with dependents: want 409, got %d body=%v", resp.StatusCode, m)
	}
	hhAssertError(t, m, "HOUSEHOLD_HAS_DEPENDENTS")
}

func TestRedGreenHousehold_PUT_MethodNotAllowed(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoMethod(t, http.MethodPut, base,
		"/api/households/00000000-0000-0000-0000-000000000001",
		`{"owner_name":"Update Attempt"}`)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("PUT household: want 405, got %d body=%v", resp.StatusCode, m)
	}
	hhAssertError(t, m, "METHOD_NOT_ALLOWED")
}

func TestRedGreenHousehold_PATCH_MethodNotAllowed(t *testing.T) {
	base := hhRedgreenBase(t)
	resp, m := hhDoMethod(t, http.MethodPatch, base,
		"/api/households/00000000-0000-0000-0000-000000000001",
		`{"owner_name":"Patch Attempt"}`)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("PATCH household: want 405, got %d body=%v", resp.StatusCode, m)
	}
	hhAssertError(t, m, "METHOD_NOT_ALLOWED")
}

func TestRedGreenHousehold_Observability_RequestID(t *testing.T) {
	base := hhRedgreenBase(t)

	resp1, _ := hhDoGet(t, base, "/api/households")
	rid1 := resp1.Header.Get("X-Request-Id")
	if rid1 == "" {
		t.Error("X-Request-Id header must be present on first GET /api/households")
	}

	resp2, _ := hhDoGet(t, base, "/api/households")
	rid2 := resp2.Header.Get("X-Request-Id")
	if rid2 == "" {
		t.Error("X-Request-Id header must be present on second GET")
	}
	if rid1 == rid2 {
		t.Errorf("consecutive requests must have different X-Request-Id; both=%s", rid1)
	}
}
