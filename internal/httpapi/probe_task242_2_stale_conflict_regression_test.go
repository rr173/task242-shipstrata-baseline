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

func bug02Call(t *testing.T, h http.Handler, method, path string, value any) (int, []byte) {
	t.Helper()
	var raw []byte
	if value != nil {
		raw, _ = json.Marshal(value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug02EntityID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func TestBug02_ReconcileClearsStaleConflict(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug02.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug02Call(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B02", "name": "Conflict lifecycle"})
	if status != http.StatusCreated {
		t.Fatalf("site: %d %s", status, body)
	}
	siteID := bug02EntityID(t, body)
	units := make([]string, 0, 3)
	for _, label := range []string{"A", "B", "C"} {
		status, body = bug02Call(t, h, http.MethodPost, "/api/units?site_id="+siteID, map[string]any{"label": label, "category": "sediment"})
		if status != http.StatusCreated {
			t.Fatalf("unit: %d %s", status, body)
		}
		units = append(units, bug02EntityID(t, body))
	}
	contacts := []string{}
	for i, pair := range [][2]string{{units[0], units[1]}, {units[1], units[2]}, {units[2], units[0]}} {
		status, body = bug02Call(t, h, http.MethodPost, "/api/contacts?site_id="+siteID, map[string]any{"from_unit_id": pair[0], "to_unit_id": pair[1], "relation": "overlies", "survey_source": "probe", "survey_seq": i + 1})
		if status != http.StatusCreated {
			t.Fatalf("contact: %d %s", status, body)
		}
		var out struct{ Contact struct{ ID string `json:"id"` } `json:"contact"` }
		if json.Unmarshal(body, &out) != nil || out.Contact.ID == "" {
			t.Fatalf("contact id: %s", body)
		}
		contacts = append(contacts, out.Contact.ID)
	}
	for _, id := range contacts {
		status, body = bug02Call(t, h, http.MethodPost, "/api/contacts/"+id+"/confirm", nil)
		if status != http.StatusOK {
			t.Fatalf("confirm: %d %s", status, body)
		}
	}
	status, body = bug02Call(t, h, http.MethodPost, "/api/contacts/"+contacts[0]+"/reject", nil)
	if status != http.StatusOK {
		t.Fatalf("reject: %d %s", status, body)
	}
	status, body = bug02Call(t, h, http.MethodGet, "/api/contacts?site_id="+siteID, nil)
	if status != http.StatusOK {
		t.Fatalf("list contacts: %d %s", status, body)
	}
	var listed []struct {
		ID     string `json:"ID"`
		Status string `json:"Status"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatal(err)
	}
	for _, c := range listed {
		if c.ID != contacts[0] && c.Status != "confirmed" {
			t.Fatalf("remaining confirmed edge retained stale derived status: %+v", listed)
		}
	}
}
