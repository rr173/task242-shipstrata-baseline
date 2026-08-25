package site

import (
	"testing"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/store"
)

func TestSiteLifecycle(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/site.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := Create(st, "S-1", "Site", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartReview(st, sb.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Publish(st, sb.ID); err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(st, sb.ID)
	if err != nil || sealed.Status != model.SiteStatusSealed || sealed.SealedAt == nil {
		t.Fatalf("seal failed: %+v err=%v", sealed, err)
	}
	if _, err := Seal(st, sb.ID); err == nil {
		t.Fatal("sealing twice should fail")
	}
}

func TestCreateUnitPreservesCategory(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/unit.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := Create(st, "S-2", "Site", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	u, err := CreateUnit(st, sb.ID, "Hull", model.UnitCategoryComponent, 1, 2, "")
	if err != nil || u.Category != model.UnitCategoryComponent {
		t.Fatalf("unit creation failed: %+v err=%v", u, err)
	}
}
