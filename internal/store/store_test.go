package store

import (
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
