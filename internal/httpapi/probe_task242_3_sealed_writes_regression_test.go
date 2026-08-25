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

func bug03HTTP(t *testing.T, h http.Handler, method, path string, body any) (int, []byte) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug03ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("invalid response: %s", body)
	}
	return out.ID
}

func TestBug03_SealedSiteRejectsAllEvidenceWrites(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug03.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug03HTTP(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B03", "name": "Sealed"})
	if status != http.StatusCreated {
		t.Fatalf("site: %d %s", status, body)
	}
	siteID := bug03ID(t, body)
	units := []string{}
	for _, label := range []string{"A", "B"} {
		status, body = bug03HTTP(t, h, http.MethodPost, "/api/units?site_id="+siteID, map[string]any{"label": label, "category": "sediment", "depth_min": 0, "depth_max": 10})
		if status != http.StatusCreated {
			t.Fatalf("unit: %d %s", status, body)
		}
		units = append(units, bug03ID(t, body))
	}
	for i := 0; i < 2; i++ {
		status, body = bug03HTTP(t, h, http.MethodPost, "/api/sites/"+siteID+"/advance", nil)
		if status != http.StatusOK {
			t.Fatalf("advance %d: %d %s", i, status, body)
		}
	}
	status, body = bug03HTTP(t, h, http.MethodPost, "/api/sites/"+siteID+"/seal", nil)
	if status != http.StatusOK {
		t.Fatalf("seal: %d %s", status, body)
	}
	for _, request := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/api/units?site_id=" + siteID, map[string]any{"label": "late", "category": "sediment"}},
		{http.MethodPost, "/api/contacts?site_id=" + siteID, map[string]any{"from_unit_id": units[0], "to_unit_id": units[1], "relation": "overlies", "survey_source": "late", "survey_seq": 1}},
		{http.MethodPost, "/api/samples?site_id=" + siteID, map[string]any{"unit_id": units[0], "label": "late sample", "depth": 5}},
	} {
		status, body = bug03HTTP(t, h, request.method, request.path, request.body)
		if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
			t.Fatalf("sealed write unexpectedly succeeded: %s status=%d body=%s", request.path, status, body)
		}
	}
	status, body = bug03HTTP(t, h, http.MethodGet, "/api/units?site_id="+siteID, nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	var unitsAfter []any
	if json.Unmarshal(body, &unitsAfter) != nil || len(unitsAfter) != 2 {
		t.Fatalf("sealed write changed unit set: %s", body)
	}
	status, body = bug03HTTP(t, h, http.MethodGet, "/api/contacts?site_id="+siteID, nil)
	var contactsAfter []any
	if status != http.StatusOK || json.Unmarshal(body, &contactsAfter) != nil || len(contactsAfter) != 0 {
		t.Fatalf("sealed write changed contacts: %d %s", status, body)
	}
	status, body = bug03HTTP(t, h, http.MethodGet, "/api/samples?site_id="+siteID, nil)
	var samplesAfter []any
	if status != http.StatusOK || json.Unmarshal(body, &samplesAfter) != nil || len(samplesAfter) != 0 {
		t.Fatalf("sealed write changed samples: %d %s", status, body)
	}
}
