package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task242-shipstrata/internal/contact"
	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/site"
	"task242-shipstrata/internal/store"
)

// buildCycle builds three units forming an overlies cycle (u1>u2>u3>u1)
// under siteID, confirms all edges, and reconciles so the site owns one open
// intrusion candidate. It returns the three unit ids (the middle unit, u2,
// is the highest-connected cycle node and thus the adjudication suspect).
func buildCycle(t *testing.T, svc *service.Services, st *store.Store, siteID, tag string) (string, string, string) {
	t.Helper()
	u1, err := site.CreateUnit(st, siteID, tag+"-1", model.UnitCategoryComponent, 10, 12, "")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := site.CreateUnit(st, siteID, tag+"-2", model.UnitCategorySediment, 8, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	u3, err := site.CreateUnit(st, siteID, tag+"-3", model.UnitCategorySediment, 0, 8, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range [][2]string{{u1.ID, u2.ID}, {u2.ID, u3.ID}, {u3.ID, u1.ID}} {
		_, c, err := contact.Import(st, siteID, e[0], e[1], model.RelOverlies, "s", 1, "")
		if err != nil {
			t.Fatalf("import contact: %v", err)
		}
		if _, err := contact.Confirm(st, c.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Strata.Reconcile(t.Context(), siteID); err != nil {
		t.Fatal(err)
	}
	return u1.ID, u2.ID, u3.ID
}

// twoSiteFixture builds two independent sites, each with its own overlies
// cycle so each owns its own open intrusion candidate. It returns the
// services, store, site A's id, site A's suspect unit, site B's id, and
// site B's suspect unit.
func twoSiteFixture(t *testing.T) (*service.Services, *store.Store, string, string, string, string) {
	t.Helper()
	st, err := store.OpenStore(t.TempDir() + "/xsite.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st)

	sbA, err := site.Create(st, "A", "Site A", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	_, aSuspect, _ := buildCycle(t, svc, st, sbA.ID, "A")

	sbB, err := site.Create(st, "B", "Site B", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	_, bSuspect, _ := buildCycle(t, svc, st, sbB.ID, "B")
	return svc, st, sbA.ID, aSuspect, sbB.ID, bSuspect
}

// openCandidateFor returns the open intrusion candidate for (siteID, unitID),
// or fails the test if there is not exactly one.
func openCandidateFor(t *testing.T, st *store.Store, siteID, unitID string) model.IntrusionCandidate {
	t.Helper()
	cands, err := st.ListIntrusionCandidates(siteID)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cands {
		if c.UnitID == unitID && c.Status == model.IntrusionOpen {
			return c
		}
	}
	t.Fatalf("no open candidate for unit %s under site %s", unitID, siteID)
	return model.IntrusionCandidate{}
}

func adjudicateRequest(h http.Handler, siteID, unitID, verdict string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"verdict": verdict})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/api/intrusion/"+unitID+"/adjudicate?site_id="+siteID, bytes.NewReader(body)))
	return rec
}

// TestAdjudicateCrossSiteIsolation asserts that submitting site B's unit via
// site A's adjudication endpoint is rejected and mutates NEITHER site B's
// unit status NOR site B's candidate status. The request must fail without
// side effects — 裁决始终受遗址范围约束.
func TestAdjudicateCrossSiteIsolation(t *testing.T) {
	svc, st, siteAID, _, siteBID, unitB := twoSiteFixture(t)

	unitBefore, _ := st.GetUnit(unitB)
	candBefore := openCandidateFor(t, st, siteBID, unitB)

	rec := adjudicateRequest(NewServer(svc).Router(), siteAID, unitB, "dismissed")
	if rec.Code == http.StatusOK {
		t.Fatalf("cross-site adjudication unexpectedly succeeded (http=%d body=%s)", rec.Code, rec.Body.String())
	}

	unitAfter, _ := st.GetUnit(unitB)
	if unitAfter.Status != unitBefore.Status {
		t.Fatalf("cross-site pollution: site B unit %s status mutated %q -> %q via site A adjudication",
			unitB, unitBefore.Status, unitAfter.Status)
	}

	candsAfter, _ := st.ListIntrusionCandidates(siteBID)
	for _, c := range candsAfter {
		if c.ID == candBefore.ID && c.Status != candBefore.Status {
			t.Fatalf("cross-site pollution: site B candidate %s status mutated %q -> %q via site A adjudication",
				c.ID, candBefore.Status, c.Status)
		}
	}
}

// TestAdjudicateSameSiteStillWorks confirms the fix does not break the
// legitimate same-site adjudication path: adjudicating a site's own suspect
// unit under that site mutates its own unit/candidate as expected.
func TestAdjudicateSameSiteStillWorks(t *testing.T) {
	svc, st, siteAID, unitA, _, _ := twoSiteFixture(t)

	rec := adjudicateRequest(NewServer(svc).Router(), siteAID, unitA, "accepted")
	if rec.Code != http.StatusOK {
		t.Fatalf("same-site adjudication failed: http=%d body=%s", rec.Code, rec.Body.String())
	}
	u, _ := st.GetUnit(unitA)
	if u.Status != model.UnitStatusDisturbed {
		t.Fatalf("same-site adjudication did not set unit to disturbed: %q", u.Status)
	}
}

// TestAdjudicateUnknownUnitRejected asserts that adjudicating a unit id that
// does not exist under the given site is rejected without side effects
// (失败不改变状态).
func TestAdjudicateUnknownUnitRejected(t *testing.T) {
	svc, _, siteAID, _, _, _ := twoSiteFixture(t)
	rec := adjudicateRequest(NewServer(svc).Router(), siteAID, "u_nonexistent", "accepted")
	if rec.Code == http.StatusOK {
		t.Fatalf("adjudication of unknown unit unexpectedly succeeded (http=%d body=%s)", rec.Code, rec.Body.String())
	}
}

// TestAdjudicateSealedSiteRejected asserts that adjudication under a sealed
// site is rejected without mutating state.
func TestAdjudicateSealedSiteRejected(t *testing.T) {
	svc, st, siteAID, unitA, _, _ := twoSiteFixture(t)
	// Advance site A through collecting -> pending_review -> published -> sealed.
	for _, step := range []string{"pending_review", "published", "sealed"} {
		var path string
		switch step {
		case "pending_review":
			path = "/advance"
		case "published":
			path = "/advance"
		case "sealed":
			path = "/seal"
		}
		rec := httptest.NewRecorder()
		NewServer(svc).Router().ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
			"/api/sites/"+siteAID+path, bytes.NewReader([]byte("{}"))))
		if rec.Code != http.StatusOK {
			t.Fatalf("advance site A to %s failed: http=%d body=%s", step, rec.Code, rec.Body.String())
		}
	}

	unitBefore, _ := st.GetUnit(unitA)
	rec := adjudicateRequest(NewServer(svc).Router(), siteAID, unitA, "accepted")
	if rec.Code == http.StatusOK {
		t.Fatalf("adjudication under sealed site unexpectedly succeeded (http=%d body=%s)", rec.Code, rec.Body.String())
	}
	unitAfter, _ := st.GetUnit(unitA)
	if unitAfter.Status != unitBefore.Status {
		t.Fatalf("sealed-site adjudication mutated unit status %q -> %q", unitBefore.Status, unitAfter.Status)
	}
}
