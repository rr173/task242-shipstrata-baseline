package sample

import (
	"testing"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/site"
	"task242-shipstrata/internal/store"
)

func TestCreateAndListSample(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/sample.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "M-1", "Samples", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	u, err := site.CreateUnit(st, sb.ID, "Layer", model.UnitCategorySediment, 0, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	sp, err := svc.Create(t.Context(), sb.ID, CreateInput{UnitID: u.ID, Label: "core", Depth: 5, Material: "sand"})
	if err != nil || sp.UnitID != u.ID {
		t.Fatalf("sample creation failed: %+v err=%v", sp, err)
	}
	all, err := svc.List(t.Context(), sb.ID)
	if err != nil || len(all) != 1 {
		t.Fatalf("sample list failed: %d err=%v", len(all), err)
	}
}
