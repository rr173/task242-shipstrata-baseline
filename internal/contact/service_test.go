package contact

import (
	"testing"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/site"
	"task242-shipstrata/internal/store"
)

func TestImportConfirmAndReject(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/contact.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "C-1", "Contact", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	a, err := site.CreateUnit(st, sb.ID, "A", model.UnitCategorySediment, 1, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := site.CreateUnit(st, sb.ID, "B", model.UnitCategorySediment, 0, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	added, c, err := Import(st, sb.ID, a.ID, b.ID, model.RelOverlies, "survey", 1, "")
	if err != nil || !added {
		t.Fatalf("import failed: added=%v err=%v", added, err)
	}
	if _, err := Confirm(st, c.ID); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetContact(c.ID)
	if err != nil || got.Status != model.ContactStatusConfirmed {
		t.Fatalf("confirm failed: %+v err=%v", got, err)
	}
	if _, err := Reject(st, c.ID); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetContact(c.ID)
	if err != nil || got.Status != model.ContactStatusExcluded {
		t.Fatalf("reject failed: %+v err=%v", got, err)
	}
}
