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

func bug01Request(t *testing.T, h http.Handler, method, path string, body any) (int, []byte) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug01ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		t.Fatalf("invalid entity response: %s", body)
	}
	return out.ID
}

func TestBug01_PendingContactsDoNotAffectGraph(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug01.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug01Request(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B01", "name": "Pending graph"})
	if status != http.StatusCreated {
		t.Fatalf("create site: %d %s", status, body)
	}
	siteID := bug01ID(t, body)
	units := make([]string, 0, 3)
	for _, label := range []string{"A", "B", "C"} {
		status, body = bug01Request(t, h, http.MethodPost, "/api/units?site_id="+siteID, map[string]any{"label": label, "category": "sediment", "depth_min": 0, "depth_max": 1})
		if status != http.StatusCreated {
			t.Fatalf("create unit: %d %s", status, body)
		}
		units = append(units, bug01ID(t, body))
	}
	contacts := make([]string, 0, 3)
	for i, pair := range [][2]string{{units[0], units[1]}, {units[1], units[2]}, {units[2], units[0]}} {
		status, body = bug01Request(t, h, http.MethodPost, "/api/contacts?site_id="+siteID, map[string]any{"from_unit_id": pair[0], "to_unit_id": pair[1], "relation": "overlies", "survey_source": "probe", "survey_seq": i + 1})
		if status != http.StatusCreated {
			t.Fatalf("create contact: %d %s", status, body)
		}
		var out struct{ Contact struct{ ID string `json:"id"` } `json:"contact"` }
		if err := json.Unmarshal(body, &out); err != nil || out.Contact.ID == "" {
			t.Fatalf("invalid contact response: %s", body)
		}
		contacts = append(contacts, out.Contact.ID)
	}
	status, body = bug01Request(t, h, http.MethodGet, "/api/graph?site_id="+siteID, nil)
	if status != http.StatusOK {
		t.Fatalf("pending graph: %d %s", status, body)
	}
	var before struct {
		IsConsistent bool     `json:"is_consistent"`
		Cycles       [][]string `json:"cycles"`
	}
	if err := json.Unmarshal(body, &before); err != nil {
		t.Fatal(err)
	}
	if !before.IsConsistent || len(before.Cycles) != 0 {
		t.Fatalf("pending contacts changed graph: %+v", before)
	}
	for _, id := range contacts {
		status, body = bug01Request(t, h, http.MethodPost, "/api/contacts/"+id+"/confirm", nil)
		if status != http.StatusOK {
			t.Fatalf("confirm contact: %d %s", status, body)
		}
	}
	status, body = bug01Request(t, h, http.MethodGet, "/api/graph?site_id="+siteID, nil)
	var after struct {
		IsConsistent bool     `json:"is_consistent"`
		Cycles       [][]string `json:"cycles"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &after) != nil {
		t.Fatalf("confirmed graph: %d %s", status, body)
	}
	if after.IsConsistent || len(after.Cycles) == 0 {
		t.Fatalf("confirmed cycle was not detected: %+v", after)
	}
}
