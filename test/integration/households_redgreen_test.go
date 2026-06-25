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

func rgHHBase(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDGREEN_BASE_URL")
	if u == "" {
		t.Skip("REDGREEN_BASE_URL not set — skipping households red-green tests")
	}
	return strings.TrimRight(u, "/")
}

func rgHHRequest(t *testing.T, method, url, contentType, body string) (*http.Response, map[string]any) {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
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

func rgHHAssertErrorCode(t *testing.T, m map[string]any, want string) {
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

func rgHHCreate(t *testing.T, base, name, addr string) string {
	t.Helper()
	body := fmt.Sprintf(`{"owner_name":%q,"address":%q}`, name, addr)
	resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("rgHHCreate: want 201, got %d body=%v", resp.StatusCode, m)
	}
	dataRaw, _ := m["data"].(map[string]any)
	id, _ := dataRaw["id"].(string)
	if id == "" {
		t.Fatal("rgHHCreate: data.id empty")
	}
	return id
}

func rgHHDelete(t *testing.T, base, id string) {
	t.Helper()
	rgHHRequest(t, http.MethodDelete, base+"/api/households/"+id, "", "")
}

func TestHouseholdsRedGreen(t *testing.T) {
	base := rgHHBase(t)

	t.Run("G01_POST_valid_201_envelope", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json",
			`{"owner_name":"RG2 Owner","address":"Jl. RG2 No. 1"}`)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("want 201, got %d", resp.StatusCode)
		}
		if resp.Header.Get("X-Request-Id") == "" {
			t.Error("X-Request-Id header missing")
		}
		dataRaw, ok := m["data"]
		if !ok {
			t.Fatal("response missing 'data'")
		}
		data, ok := dataRaw.(map[string]any)
		if !ok {
			t.Fatalf("data must be object, got %T", dataRaw)
		}
		for _, f := range []string{"id", "owner_name", "address", "created_at", "updated_at"} {
			if _, has := data[f]; !has {
				t.Errorf("data missing field %q", f)
			}
		}
		if _, hasMeta := m["meta"]; hasMeta {
			t.Error("meta must NOT be present on create response")
		}
		id, _ := data["id"].(string)
		rgHHDelete(t, base, id)
	})

	t.Run("G02_GET_list_200_with_meta", func(t *testing.T) {
		id := rgHHCreate(t, base, "RG2 List Owner", "Jl. RG2 List No. 1")
		defer rgHHDelete(t, base, id)

		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		dataRaw, ok := m["data"]
		if !ok {
			t.Fatal("response missing 'data'")
		}
		if _, ok := dataRaw.([]any); !ok {
			t.Fatalf("data must be array, got %T", dataRaw)
		}
		metaRaw, ok := m["meta"]
		if !ok {
			t.Fatal("response missing 'meta'")
		}
		meta, ok := metaRaw.(map[string]any)
		if !ok {
			t.Fatalf("meta must be object, got %T", metaRaw)
		}
		for _, key := range []string{"page", "per_page", "total"} {
			if _, has := meta[key]; !has {
				t.Errorf("meta missing key %q", key)
			}
		}
	})

	t.Run("G03_GET_list_pagination_page2_per_page3", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households?page=2&per_page=3", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		meta, _ := m["meta"].(map[string]any)
		if page, _ := meta["page"].(float64); page != 2 {
			t.Errorf("meta.page: want 2, got %v", page)
		}
		if pp, _ := meta["per_page"].(float64); pp != 3 {
			t.Errorf("meta.per_page: want 3, got %v", pp)
		}
	})

	t.Run("G04_GET_by_id_200_no_meta", func(t *testing.T) {
		id := rgHHCreate(t, base, "RG2 GetByID", "Jl. RG2 GetByID No. 1")
		defer rgHHDelete(t, base, id)

		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households/"+id, "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if resp.Header.Get("X-Request-Id") == "" {
			t.Error("X-Request-Id missing on GET by id")
		}
		data, _ := m["data"].(map[string]any)
		if gotID, _ := data["id"].(string); gotID != id {
			t.Errorf("data.id: want %q, got %q", id, gotID)
		}
		if _, hasMeta := m["meta"]; hasMeta {
			t.Error("meta must NOT be present on single-resource response")
		}
	})

	t.Run("G05_DELETE_disposable_204_no_body", func(t *testing.T) {
		id := rgHHCreate(t, base, "RG2 Delete Me", "Jl. RG2 Delete No. 1")
		resp, m := rgHHRequest(t, http.MethodDelete, base+"/api/households/"+id, "", "")
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("want 204, got %d", resp.StatusCode)
		}
		if len(m) != 0 {
			t.Errorf("204 body must be empty; got %v", m)
		}
	})

	t.Run("G06_DELETE_with_dependents_409", func(t *testing.T) {
		hhID := rgHHCreate(t, base, "RG2 Dependent", "Jl. RG2 Dependent No. 1")

		pickupBody := fmt.Sprintf(`{"household_id":%q,"type":"organic"}`, hhID)
		pResp, _ := rgHHRequest(t, http.MethodPost, base+"/api/pickups", "application/json", pickupBody)
		if pResp.StatusCode != http.StatusCreated {
			t.Fatalf("create pickup for dependent: want 201, got %d", pResp.StatusCode)
		}

		resp, m := rgHHRequest(t, http.MethodDelete, base+"/api/households/"+hhID, "", "")
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("want 409, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "HOUSEHOLD_HAS_DEPENDENTS")
	})

	t.Run("G07_POST_boundary_owner_name_255_valid", func(t *testing.T) {
		long255 := strings.Repeat("C", 255)
		body := fmt.Sprintf(`{"owner_name":%q,"address":"Jl. Boundary"}`, long255)
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("owner_name=255 chars: want 201, got %d", resp.StatusCode)
		}
		data, _ := m["data"].(map[string]any)
		id, _ := data["id"].(string)
		rgHHDelete(t, base, id)
	})

	t.Run("G08_POST_boundary_address_1000_valid", func(t *testing.T) {
		long1000 := strings.Repeat("D", 1000)
		body := fmt.Sprintf(`{"owner_name":"Boundary","address":%q}`, long1000)
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("address=1000 chars: want 201, got %d", resp.StatusCode)
		}
		data, _ := m["data"].(map[string]any)
		id, _ := data["id"].(string)
		rgHHDelete(t, base, id)
	})

	t.Run("R01_POST_missing_owner_name_400", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json",
			`{"address":"Jl. Test"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "owner_name" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=owner_name")
		}
	})

	t.Run("R02_POST_missing_address_400", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json",
			`{"owner_name":"Test"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		found := false
		for _, d := range details {
			dm, _ := d.(map[string]any)
			if dm["field"] == "address" {
				found = true
			}
		}
		if !found {
			t.Error("details must include field=address")
		}
	})

	t.Run("R03_POST_owner_name_over_255_400", func(t *testing.T) {
		long := strings.Repeat("A", 256)
		body := fmt.Sprintf(`{"owner_name":%q,"address":"Jl. Test"}`, long)
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("owner_name=256: want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R04_POST_address_over_1000_400", func(t *testing.T) {
		long := strings.Repeat("B", 1001)
		body := fmt.Sprintf(`{"owner_name":"Test","address":%q}`, long)
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("address=1001: want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R05_POST_empty_body_400_both_fields", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", `{}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("empty body: want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
		errMap, _ := m["error"].(map[string]any)
		details, _ := errMap["details"].([]any)
		if len(details) < 2 {
			t.Errorf("both missing fields must appear in details; got %d entries", len(details))
		}
	})

	t.Run("R06_POST_malformed_json_400_not_500", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json", `{bad json}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("malformed JSON: want 400 (not 500), got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R07_POST_wrong_content_type_400", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "text/plain",
			`{"owner_name":"Test","address":"Jl."}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("text/plain Content-Type: want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R08_POST_whitespace_owner_name_400", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPost, base+"/api/households", "application/json",
			`{"owner_name":"   ","address":"Jl. Test"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("whitespace-only owner_name: want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R09_GET_by_id_non_uuid_400_not_404", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households/not-a-uuid", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("non-UUID id: want 400 (not 404/500), got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
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

	t.Run("R10_GET_by_id_unknown_uuid_404", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households/00000000-0000-0000-0000-000000000099", "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("unknown UUID: want 404, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "HOUSEHOLD_NOT_FOUND")
	})

	t.Run("R11_DELETE_non_uuid_400", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodDelete, base+"/api/households/not-a-uuid", "", "")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("non-UUID id: want 400, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "VALIDATION_ERROR")
	})

	t.Run("R12_DELETE_unknown_uuid_404", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodDelete, base+"/api/households/00000000-0000-0000-0000-000000000099", "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("unknown UUID: want 404, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "HOUSEHOLD_NOT_FOUND")
	})

	t.Run("R13_PATCH_method_not_allowed_405", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodPatch, base+"/api/households/00000000-0000-0000-0000-000000000001",
			"application/json", `{"owner_name":"Patch"}`)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("PATCH: want 405, got %d", resp.StatusCode)
		}
		rgHHAssertErrorCode(t, m, "METHOD_NOT_ALLOWED")
	})

	t.Run("R14_GET_list_per_page_0_silently_defaults", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households?per_page=0", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("per_page=0: want 200 (silent default), got %d", resp.StatusCode)
		}
		if _, hasMeta := m["meta"]; !hasMeta {
			t.Error("meta missing on list response")
		}
	})

	t.Run("R15_GET_list_page_non_integer_silently_defaults", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households?page=abc", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("page=abc: want 200 (silent default), got %d", resp.StatusCode)
		}
		if _, hasMeta := m["meta"]; !hasMeta {
			t.Error("meta missing on list response")
		}
	})

	t.Run("R16_GET_list_per_page_cap_at_100", func(t *testing.T) {
		resp, m := rgHHRequest(t, http.MethodGet, base+"/api/households?per_page=200", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("per_page=200: want 200, got %d", resp.StatusCode)
		}
		meta, _ := m["meta"].(map[string]any)
		perPage, _ := meta["per_page"].(float64)
		if perPage > 100 {
			t.Errorf("per_page should be capped at 100, got %v", perPage)
		}
	})

	t.Run("OBS01_X_Request_Id_unique_per_request", func(t *testing.T) {
		resp1, _ := rgHHRequest(t, http.MethodGet, base+"/api/households", "", "")
		rid1 := resp1.Header.Get("X-Request-Id")
		if rid1 == "" {
			t.Error("X-Request-Id must be present on first request")
		}
		resp2, _ := rgHHRequest(t, http.MethodGet, base+"/api/households", "", "")
		rid2 := resp2.Header.Get("X-Request-Id")
		if rid2 == "" {
			t.Error("X-Request-Id must be present on second request")
		}
		if rid1 == rid2 {
			t.Errorf("consecutive requests must have unique X-Request-Id; both=%q", rid1)
		}
	})
}
