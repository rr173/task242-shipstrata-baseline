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

func TestRouterCreatesSiteAndListsIt(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/http.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()
	body, _ := json.Marshal(map[string]string{"code": "H-1", "name": "HTTP Site"})
	create := httptest.NewRecorder()
	h.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/sites", bytes.NewReader(body)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	list := httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/sites", nil))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte("HTTP Site")) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
}
