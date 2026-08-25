package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/store"
)

func bug06Request(t *testing.T, h http.Handler, method, path string, body any) (int, []byte) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	return rec.Code, rec.Body.Bytes()
}

func bug06ID(t *testing.T, body []byte) string {
	t.Helper()
	var out struct{ ID string `json:"id"` }
	if json.Unmarshal(body, &out) != nil || out.ID == "" {
		t.Fatalf("missing id: %s", body)
	}
	return out.ID
}

func bug06PublishWithBusyRetry(t *testing.T, h http.Handler, siteID string) (int, []byte) {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		status, body := bug06Request(t, h, http.MethodPost, "/api/profiles?site_id="+siteID, map[string]string{"label": "v"})
		if status != http.StatusBadRequest || !bytes.Contains(body, []byte("database is locked")) {
			return status, body
		}
		time.Sleep(time.Millisecond)
	}
	return http.StatusServiceUnavailable, []byte("database remained locked")
}

func TestBug06_ConcurrentProfileVersions(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/bug06.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	status, body := bug06Request(t, h, http.MethodPost, "/api/sites", map[string]string{"code": "B06", "name": "Concurrent profiles"})
	if status != http.StatusCreated {
		t.Fatal(status, string(body))
	}
	siteID := bug06ID(t, body)
	const participants = 20
	start := make(chan struct{})
	results := make(chan struct {
		status int
		body   []byte
	}, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			status, body := bug06PublishWithBusyRetry(t, h, siteID)
			results <- struct {
				status int
				body   []byte
			}{status, body}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	versions := make([]int, 0, participants)
	for result := range results {
		if result.status != http.StatusCreated {
			t.Fatalf("concurrent publish failed: %d %s", result.status, result.body)
		}
		var out struct{ Profile struct{ Version int `json:"Version"` } `json:"profile"` }
		if json.Unmarshal(result.body, &out) != nil || out.Profile.Version == 0 {
			t.Fatalf("invalid publish response: %s", result.body)
		}
		versions = append(versions, out.Profile.Version)
	}
	sort.Ints(versions)
	for i, version := range versions {
		if version != i+1 {
			t.Fatalf("profile versions are not unique and contiguous: %v", versions)
		}
	}
}
