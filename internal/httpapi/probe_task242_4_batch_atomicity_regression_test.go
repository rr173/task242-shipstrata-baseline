package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/store"
)

func bug04Do(t *testing.T, h http.Handler, method, path string, value any) (int, []byte) {
	t.Helper()
	var raw []byte
	if value != nil {
		raw, _ = json.Marshal(value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug04ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("invalid id response: %s", body)
	}
	return out.ID
}

func TestBug04_BatchContactImportIsAtomic(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug04.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug04Do(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B04", "name": "Batch"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	siteID := bug04ID(t, body)
	units := []string{}
	for _, label := range []string{"A", "B", "C"} {
		status, body = bug04Do(t, h, http.MethodPost, "/api/units?site_id="+siteID, map[string]any{"label": label, "category": "sediment"})
		if status != http.StatusCreated {
			t.Fatal(status, string(body))
		}
		units = append(units, bug04ID(t, body))
	}
	batch := map[string]any{"contacts": []any{
		map[string]any{"from_unit_id": units[0], "to_unit_id": units[1], "relation": "overlies", "survey_source": "batch", "survey_seq": 1},
		map[string]any{"from_unit_id": units[1], "to_unit_id": units[2], "relation": "overlies", "survey_source": "batch", "survey_seq": 2},
		map[string]any{"from_unit_id": units[2], "to_unit_id": units[2], "relation": "overlies", "survey_source": "batch", "survey_seq": 3},
	}}
	status, body = bug04Do(t, h, http.MethodPost, "/api/contacts/batch?site_id="+siteID, batch)
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		t.Fatalf("invalid batch unexpectedly succeeded: %d %s", status, body)
	}
	status, body = bug04Do(t, h, http.MethodGet, "/api/contacts?site_id="+siteID, nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	var contacts []any
	if err := json.Unmarshal(body, &contacts); err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 0 {
		t.Fatalf("failed batch partially committed %d contacts: %s", len(contacts), body)
	}
}
