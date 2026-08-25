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

func TestBatchContactsAtomicity(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/http_batch.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := NewServer(service.New(st)).Router()

	// 建遗址与两个单元 A、B。
	siteBody, _ := json.Marshal(map[string]string{"code": "W-1", "name": "Wreck"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/sites", bytes.NewReader(siteBody)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create site: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var siteResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &siteResp); err != nil {
		t.Fatal(err)
	}
	siteID := siteResp.ID

	mkUnit := func(label string) string {
		b, _ := json.Marshal(map[string]any{"label": label, "category": "sediment", "depth_min": 0, "depth_max": 1})
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/units?site_id="+siteID, bytes.NewReader(b)))
		if r.Code != http.StatusCreated {
			t.Fatalf("create unit %s: status=%d body=%s", label, r.Code, r.Body.String())
		}
		var u struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &u); err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	aID := mkUnit("A")
	bID := mkUnit("B")

	// 前两条有效、第三条引用未知单元（非法）。接口应返回错误且一条也不写入。
	batch := map[string]any{"contacts": []map[string]any{
		{"from_unit_id": aID, "to_unit_id": bID, "relation": "overlies", "survey_source": "survey", "survey_seq": 1},
		{"from_unit_id": aID, "to_unit_id": bID, "relation": "cuts", "survey_source": "survey", "survey_seq": 2},
		{"from_unit_id": aID, "to_unit_id": "unit_does_not_exist", "relation": "overlies", "survey_source": "survey", "survey_seq": 3},
	}}
	batchBody, _ := json.Marshal(batch)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/contacts/batch?site_id="+siteID, bytes.NewReader(batchBody)))
	if rec.Code == http.StatusOK {
		t.Fatalf("expected error status, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 失败批次不应残留任何接触关系。
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/contacts?site_id="+siteID, nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list contacts: status=%d", listRec.Code)
	}
	if bytes.Contains(listRec.Body.Bytes(), []byte(`"id":`)) {
		t.Fatalf("expected zero committed contacts after failed batch, got body=%s", listRec.Body.String())
	}

	// 修正第三条后整批重放，三条应全部写入（可安全重放）。
	batch["contacts"].([]map[string]any)[2]["to_unit_id"] = bID
	batchBody, _ = json.Marshal(batch)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/contacts/batch?site_id="+siteID, bytes.NewReader(batchBody)))
	if rec.Code != http.StatusOK {
		t.Fatalf("replay batch: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Added   int `json:"added"`
		Skipped int `json:"skipped"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Added != 3 || resp.Skipped != 0 {
		t.Fatalf("expected 3 added on replay, got added=%d skipped=%d", resp.Added, resp.Skipped)
	}
}

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
