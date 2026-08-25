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

func bug08Request(t *testing.T, h http.Handler, method, path string, value any) (int, []byte) {
	t.Helper()
	var raw []byte
	if value != nil {
		raw, _ = json.Marshal(value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug08ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func TestBug08_FrozenProfileSurvivesRestart(t *testing.T) {
	db := t.TempDir() + "/bug08.db"
	st, err := store.OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	h := NewServer(service.New(st)).Router()
	status, body := bug08Request(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B08", "name": "Frozen"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	siteID := bug08ID(t, body)
	status, body = bug08Request(t, h, http.MethodPost, "/api/profiles?site_id="+siteID, map[string]string{"label": "frozen-v1"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	var created struct{ Profile struct{ ID string `json:"ID"`; Snapshot string `json:"Snapshot"` } `json:"profile"` }
	if json.Unmarshal(body, &created) != nil || created.Profile.ID == "" {
		t.Fatalf("invalid profile: %s", body)
	}
	profileID := created.Profile.ID
	status, body = bug08Request(t, h, http.MethodPost, "/api/profiles/"+profileID+"/share", nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	status, body = bug08Request(t, h, http.MethodPost, "/api/profiles/"+profileID+"/freeze", nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"Status":"frozen"`)) || bytes.Contains(body, []byte(`"FrozenAt":null`)) {
		t.Fatalf("freeze response lost immutable metadata: %d %s", status, body)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.OpenStore(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h = NewServer(service.New(st)).Router()
	status, body = bug08Request(t, h, http.MethodGet, "/api/profiles/"+profileID, nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"Status":"frozen"`)) || bytes.Contains(body, []byte(`"FrozenAt":null`)) {
		t.Fatalf("restart lost frozen metadata: %d %s", status, body)
	}
}
