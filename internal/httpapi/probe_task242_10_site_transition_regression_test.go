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

func bug10Request(t *testing.T, h http.Handler, method, path string) (int, []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(nil)))
	return rec.Code, rec.Body.Bytes()
}

func bug10ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func bug10Concurrent(t *testing.T, h http.Handler, method, path string) map[int]int {
	t.Helper()
	const participants = 20
	start := make(chan struct{})
	results := make(chan int, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, _ := bug10Request(t, h, method, path)
			results <- status
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	counts := make(map[int]int)
	for status := range results {
		counts[status]++
	}
	return counts
}

func TestBug10_SiteTransitionsAreCompareAndSet(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug10.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	raw := bytes.NewBufferString(`{"code":"B10","name":"CAS"}`)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/sites", raw))
	if rec.Code != http.StatusCreated {
		t.Fatalf("site: %d %s", rec.Code, rec.Body.Bytes())
	}
	siteID := bug10ID(t, rec.Body.Bytes())

	advance := bug10Concurrent(t, h, http.MethodPost, "/api/sites/"+siteID+"/advance")
	if advance[http.StatusOK] != 2 || advance[http.StatusConflict] == 0 {
		t.Fatalf("advance transitions were not compare-and-set: %#v", advance)
	}

	seal := bug10Concurrent(t, h, http.MethodPost, "/api/sites/"+siteID+"/seal")
	if seal[http.StatusOK] != 1 {
		t.Fatalf("seal transitions were not compare-and-set: %#v", seal)
	}

	status, body := bug10Request(t, h, http.MethodGet, "/api/sites/"+siteID)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"Status":"sealed"`)) || !bytes.Contains(body, []byte(`"Version":4`)) {
		t.Fatalf("lifecycle state/version is not linearizable: %d %s", status, body)
	}
}
