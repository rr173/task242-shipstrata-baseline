package store

import (
	"errors"
	"testing"

	"task242-shipstrata/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(t.TempDir() + "/shipstrata.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestStoreRoundTripSiteAndUnit(t *testing.T) {
	st := openTestStore(t)
	site := &model.SiteBatch{ID: "sb_test", Code: "T-1", Name: "Test", Status: model.SiteStatusCollecting, Version: 1}
	if err := st.CreateSite(site); err != nil {
		t.Fatalf("create site: %v", err)
	}
	u := &model.StrataUnit{ID: "u_test", SiteID: site.ID, Label: "Layer", Category: model.UnitCategorySediment, Status: model.UnitStatusCandidate, Version: 1}
	if err := st.CreateUnit(u); err != nil {
		t.Fatalf("create unit: %v", err)
	}
	got, err := st.GetUnit(u.ID)
	if err != nil || got.SiteID != site.ID || got.Label != u.Label {
		t.Fatalf("round trip mismatch: got=%+v err=%v", got, err)
	}
}

func TestContactFingerprintIsIdempotent(t *testing.T) {
	st := openTestStore(t)
	site := &model.SiteBatch{ID: "sb_contact", Code: "T-2", Name: "Test", Status: model.SiteStatusCollecting, Version: 1}
	if err := st.CreateSite(site); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"u_a", "u_b"} {
		if err := st.CreateUnit(&model.StrataUnit{ID: id, SiteID: site.ID, Label: id, Category: model.UnitCategorySediment, Status: model.UnitStatusCandidate, Version: 1}); err != nil {
			t.Fatal(err)
		}
	}
	c := &model.Contact{ID: "c_contact", SiteID: site.ID, FromUnitID: "u_a", ToUnitID: "u_b", Relation: model.RelOverlies, Status: model.ContactStatusPending, Fingerprint: "fp_contact", Version: 1}
	added, err := st.CreateContact(c)
	if err != nil || !added {
		t.Fatalf("first insert: added=%v err=%v", added, err)
	}
	added, err = st.CreateContact(&model.Contact{ID: "c_other", SiteID: site.ID, FromUnitID: "u_a", ToUnitID: "u_b", Relation: model.RelOverlies, Status: model.ContactStatusPending, Fingerprint: "fp_contact", Version: 1})
	if err != nil || added {
		t.Fatalf("duplicate insert should be ignored: added=%v err=%v", added, err)
	}
}

// TestSupersedeProfileRejectsCrossSiteAndPreservesStatus verifies the store
// layer refuses to supersede an old profile with a draft from another site and
// leaves both statuses untouched.
func TestSupersedeProfileRejectsCrossSiteAndPreservesStatus(t *testing.T) {
	st := openTestStore(t)
	for _, sid := range []string{"sb_old", "sb_new"} {
		if err := st.CreateSite(&model.SiteBatch{ID: sid, Code: sid, Name: sid, Status: model.SiteStatusCollecting, Version: 1}); err != nil {
			t.Fatal(err)
		}
	}
	old := &model.ProfileVersion{ID: "prf_old", SiteID: "sb_old", Version: 1, Status: model.ProfileStatusFrozen}
	newP := &model.ProfileVersion{ID: "prf_new", SiteID: "sb_new", Version: 1, Status: model.ProfileStatusDraft}
	for _, p := range []*model.ProfileVersion{old, newP} {
		if err := st.CreateProfile(p); err != nil {
			t.Fatalf("create profile %s: %v", p.ID, err)
		}
	}
	if err := st.SupersedeProfile(old.ID, newP.ID); !errors.Is(err, model.ErrCrossSite) {
		t.Fatalf("expected ErrCrossSite, got %v", err)
	}
	if got, err := st.GetProfile(old.ID); err != nil || got.Status != model.ProfileStatusFrozen {
		t.Fatalf("old status changed: %+v err=%v", got, err)
	}
	if got, err := st.GetProfile(newP.ID); err != nil || got.Status != model.ProfileStatusDraft {
		t.Fatalf("new status changed: %+v err=%v", got, err)
	}
}
