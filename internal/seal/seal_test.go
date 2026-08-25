package seal

import (
	"errors"
	"testing"

	"task242-shipstrata/internal/contact"
	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/sample"
	"task242-shipstrata/internal/site"
	"task242-shipstrata/internal/store"
)

// sealStore 返回一个已封存的遗址批次与封存前建好的两个地层单元，
// 用于断言封存后再写入证据必须被拒绝。
func sealStore(t *testing.T) (*store.Store, *model.SiteBatch, *model.StrataUnit, *model.StrataUnit) {
	st, err := store.OpenStore(t.TempDir() + "/seal.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	sb, err := site.Create(st, "SEAL-1", "Sealed Site", "offshore")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	u, err := site.CreateUnit(st, sb.ID, "Hull", model.UnitCategoryComponent, 1, 2, "")
	if err != nil {
		t.Fatalf("create unit before seal: %v", err)
	}
	u2, err := site.CreateUnit(st, sb.ID, "Ballast", model.UnitCategorySediment, 0, 1, "")
	if err != nil {
		t.Fatalf("create second unit before seal: %v", err)
	}
	if sb, err = site.StartReview(st, sb.ID); err != nil {
		t.Fatalf("start review: %v", err)
	}
	if sb, err = site.Publish(st, sb.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if sb, err = site.Seal(st, sb.ID); err != nil {
		t.Fatalf("seal: %v", err)
	}
	return st, sb, u, u2
}

// assertSealed 检测 err 是否为封存拒绝错误。
func assertSealed(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s after seal must be rejected", what)
	}
	if !errors.Is(err, model.ErrSiteSealed) {
		t.Fatalf("%s after seal must report ErrSiteSealed, got %v", what, err)
	}
}

func TestSealRejectsNewUnit(t *testing.T) {
	st, sb, _, _ := sealStore(t)
	before, _ := st.ListUnits(sb.ID)
	if _, err := site.CreateUnit(st, sb.ID, "Latecomer", model.UnitCategorySediment, 0, 1, ""); err == nil {
		t.Fatal("creating a unit after seal must be rejected")
	}
	after, _ := st.ListUnits(sb.ID)
	if len(after) != len(before) {
		t.Fatalf("sealed site gained a unit: before=%d after=%d", len(before), len(after))
	}
}

func TestSealRejectsSingleContactImport(t *testing.T) {
	st, sb, u, u2 := sealStore(t)
	before, _ := st.ListContacts(sb.ID)
	added, c, err := contact.Import(st, sb.ID, u.ID, u2.ID, model.RelOverlies, "survey", 1, "")
	if err == nil {
		t.Fatalf("importing a contact after seal must be rejected (added=%v)", added)
	}
	if c != nil {
		t.Fatalf("rejected contact must not return a record, got %+v", c)
	}
	after, _ := st.ListContacts(sb.ID)
	if len(after) != len(before) {
		t.Fatalf("sealed site gained a contact: before=%d after=%d", len(before), len(after))
	}
}

func TestSealRejectsBatchContactImport(t *testing.T) {
	st, sb, u, u2 := sealStore(t)
	before, _ := st.ListContacts(sb.ID)
	_, _, err := contact.ImportBatch(st, sb.ID, []contact.ImportInput{
		{FromUnitID: u.ID, ToUnitID: u2.ID, Relation: model.RelOverlies, SurveySource: "survey", SurveySeq: 1},
	})
	assertSealed(t, err, "batch contact import")
	after, _ := st.ListContacts(sb.ID)
	if len(after) != len(before) {
		t.Fatalf("sealed site gained a contact: before=%d after=%d", len(before), len(after))
	}
}

func TestSealRejectsSample(t *testing.T) {
	st, sb, u, _ := sealStore(t)
	svc := sample.NewService(st)
	before, _ := st.ListSamples(sb.ID)
	_, err := svc.Create(t.Context(), sb.ID, sample.CreateInput{
		UnitID: u.ID, Label: "core", Depth: 1.5, Material: "wood",
	})
	assertSealed(t, err, "sample creation")
	after, _ := st.ListSamples(sb.ID)
	if len(after) != len(before) {
		t.Fatalf("sealed site gained a sample: before=%d after=%d", len(before), len(after))
	}
}
