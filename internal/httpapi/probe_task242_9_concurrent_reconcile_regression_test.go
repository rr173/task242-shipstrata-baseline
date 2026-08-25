package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/store"
)

func bug09Call(t *testing.T, h http.Handler, method, path string, value any) (int, []byte) {
	t.Helper()
	var raw []byte
	if value != nil {
		raw, _ = json.Marshal(value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug09ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func TestBug09_ConcurrentReconcileHasOneCurrentFinding(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug09.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug09Call(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B09", "name": "Concurrent reconcile"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	siteID := bug09ID(t, body)
	units := []string{}
	for _, label := range []string{"A", "B", "C"} {
		status, body = bug09Call(t, h, http.MethodPost, "/api/units?site_id="+siteID, map[string]any{"label": label, "category": "sediment"})
		if status != http.StatusCreated {
			t.Fatal(status, string(body))
		}
		units = append(units, bug09ID(t, body))
	}
	for i, pair := range [][2]string{{units[0], units[1]}, {units[1], units[2]}, {units[2], units[0]}} {
		status, body = bug09Call(t, h, http.MethodPost, "/api/contacts?site_id="+siteID, map[string]any{"from_unit_id": pair[0], "to_unit_id": pair[1], "relation": "overlies", "survey_source": "probe", "survey_seq": i + 1})
		if status != http.StatusCreated {
			t.Fatal(status, string(body))
		}
		var out struct{ Contact struct{ ID string `json:"id"` } `json:"contact"` }
		json.Unmarshal(body, &out)
		status, body = bug09Call(t, h, http.MethodPost, "/api/contacts/"+out.Contact.ID+"/confirm", nil)
		if status != http.StatusOK {
			t.Fatal(status, string(body))
		}
	}
	const participants = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan int, participants)
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, _ := bug09Call(t, h, http.MethodPost, "/api/intrusion/detect?site_id="+siteID, nil)
			results <- status
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for status := range results {
		if status != http.StatusOK {
			t.Fatalf("concurrent reconcile failed with status %d", status)
		}
	}
	status, body = bug09Call(t, h, http.MethodGet, "/api/contradictions?site_id="+siteID, nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	var contradictions []struct{ Resolved bool `json:"Resolved"` }
	if json.Unmarshal(body, &contradictions) != nil {
		t.Fatal(string(body))
	}
	openContradictions := 0
	for _, c := range contradictions {
		if !c.Resolved {
			openContradictions++
		}
	}
	if openContradictions != 1 {
		t.Fatalf("expected one unresolved contradiction, got %d in %s", openContradictions, body)
	}
	status, body = bug09Call(t, h, http.MethodGet, "/api/intrusion/candidates?site_id="+siteID, nil)
	if status != http.StatusOK {
		t.Fatal(status, string(body))
	}
	var candidates []struct{ Status string `json:"Status"` }
	if json.Unmarshal(body, &candidates) != nil {
		t.Fatal(string(body))
	}
	openCandidates := 0
	for _, c := range candidates {
		if c.Status == "open" {
			openCandidates++
		}
	}
	if openCandidates != 1 {
		t.Fatalf("expected one open intrusion candidate, got %d in %s", openCandidates, body)
	}
}
