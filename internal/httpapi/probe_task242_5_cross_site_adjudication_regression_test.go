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

func bug05Call(t *testing.T, h http.Handler, method, path string, body any) (int, []byte) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug05ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func TestBug05_AdjudicationCannotCrossSite(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug05.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug05Call(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "A05", "name": "Site A"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	aSite := bug05ID(t, body)
	status, body = bug05Call(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B05", "name": "Site B"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	bSite := bug05ID(t, body)
	aUnits := []string{}
	for _, label := range []string{"A", "B", "C"} {
		status, body = bug05Call(t, h, http.MethodPost, "/api/units?site_id="+aSite, map[string]any{"label": label, "category": "sediment"})
		if status != http.StatusCreated {
			t.Fatal(status, string(body))
		}
		aUnits = append(aUnits, bug05ID(t, body))
	}
	for i, pair := range [][2]string{{aUnits[0], aUnits[1]}, {aUnits[1], aUnits[2]}, {aUnits[2], aUnits[0]}} {
		status, body = bug05Call(t, h, http.MethodPost, "/api/contacts?site_id="+aSite, map[string]any{"from_unit_id": pair[0], "to_unit_id": pair[1], "relation": "overlies", "survey_source": "probe", "survey_seq": i + 1})
		if status != http.StatusCreated {
			t.Fatal(status, string(body))
		}
		var contact struct{ Contact struct{ ID string `json:"id"` } `json:"contact"` }
		json.Unmarshal(body, &contact)
		status, body = bug05Call(t, h, http.MethodPost, "/api/contacts/"+contact.Contact.ID+"/confirm", nil)
		if status != http.StatusOK {
			t.Fatal(status, string(body))
		}
	}
	status, body = bug05Call(t, h, http.MethodGet, "/api/intrusion/candidates?site_id="+aSite, nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	var candidates []struct{ UnitID string `json:"UnitID"` }
	if json.Unmarshal(body, &candidates) != nil || len(candidates) == 0 {
		t.Fatalf("candidate missing: %s", body)
	}
	unitID := candidates[0].UnitID
	status, body = bug05Call(t, h, http.MethodPost, "/api/intrusion/"+unitID+"/adjudicate?site_id="+bSite, map[string]string{"verdict": "accepted"})
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		t.Fatalf("cross-site adjudication unexpectedly succeeded: %d %s", status, body)
	}
	status, body = bug05Call(t, h, http.MethodGet, "/api/units/"+unitID, nil)
	if status != http.StatusOK || bytes.Contains(body, []byte(`"Status":"disturbed"`)) {
		t.Fatalf("cross-site request changed Site A unit: %d %s", status, body)
	}
}
