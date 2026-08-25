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

func bug07HTTP(t *testing.T, h http.Handler, method, path string, value any) (int, []byte) {
	t.Helper()
	var raw []byte
	if value != nil {
		raw, _ = json.Marshal(value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug07ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func bug07Profile(t *testing.T, body []byte) (string, string) {
	t.Helper()
	var out struct{ Profile struct{ ID string `json:"ID"`; Status string `json:"Status"` } `json:"profile"` }
	if json.Unmarshal(body, &out) != nil || out.Profile.ID == "" {
		t.Fatalf("missing profile: %s", body)
	}
	return out.Profile.ID, out.Profile.Status
}

func TestBug07_SupersedeCannotCrossSite(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug07.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug07HTTP(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "A07", "name": "A"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	aSite := bug07ID(t, body)
	status, body = bug07HTTP(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B07", "name": "B"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	bSite := bug07ID(t, body)
	status, body = bug07HTTP(t, h, http.MethodPost, "/api/profiles?site_id="+aSite, map[string]string{"label": "A1"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	oldID, _ := bug07Profile(t, body)
	status, body = bug07HTTP(t, h, http.MethodPost, "/api/profiles/"+oldID+"/share", nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	status, body = bug07HTTP(t, h, http.MethodPost, "/api/profiles?site_id="+bSite, map[string]string{"label": "B1"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	newID, _ := bug07Profile(t, body)
	status, body = bug07HTTP(t, h, http.MethodPost, "/api/profiles/"+oldID+"/supersede", map[string]string{"new_id": newID})
	if status < http.StatusBadRequest || status >= http.StatusInternalServerError {
		t.Fatalf("cross-site supersede unexpectedly succeeded: %d %s", status, body)
	}
	status, body = bug07HTTP(t, h, http.MethodGet, "/api/profiles/"+oldID, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"Status":"shared"`)) {
		t.Fatalf("old profile changed across site boundary: %d %s", status, body)
	}
	status, body = bug07HTTP(t, h, http.MethodGet, "/api/profiles/"+newID, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"Status":"draft"`)) {
		t.Fatalf("new profile changed across site boundary: %d %s", status, body)
	}
}
