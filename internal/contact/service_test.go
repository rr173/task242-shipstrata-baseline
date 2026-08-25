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

// TestImportBatchAtomicity 验证批量导入的「全写或全不写」不变量：
// 前两条有效、第三条非法时，整批必须失败且一条也不写入，使得批次可安全重放。
func TestImportBatchAtomicity(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/contact_batch.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "B-1", "Batch", "offshore")
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
	// 前两条有效，第三条引用未知单元（非法）。
	inputs := []ImportInput{
		{FromUnitID: a.ID, ToUnitID: b.ID, Relation: model.RelOverlies, SurveySource: "survey", SurveySeq: 1},
		{FromUnitID: a.ID, ToUnitID: b.ID, Relation: model.RelCuts, SurveySource: "survey", SurveySeq: 2},
		{FromUnitID: a.ID, ToUnitID: "unit_does_not_exist", Relation: model.RelOverlies, SurveySource: "survey", SurveySeq: 3},
	}
	added, skipped, err := ImportBatch(st, sb.ID, inputs)
	if err == nil {
		t.Fatalf("expected error for invalid batch, got added=%d skipped=%d", added, skipped)
	}
	if added != 0 || skipped != 0 {
		t.Fatalf("expected 0 added/0 skipped on failed batch, got added=%d skipped=%d", added, skipped)
	}
	// 整批失败后库中不应存在任何本批次接触关系。
	cs, err := st.ListContacts(sb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 0 {
		t.Fatalf("expected no committed contacts after failed batch, got %d", len(cs))
	}
	// 修正第三条后整批重放：两条全部写入，批次可安全重放。
	inputs[2].ToUnitID = b.ID
	added, skipped, err = ImportBatch(st, sb.ID, inputs)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if added != 3 || skipped != 0 {
		t.Fatalf("expected 3 added on replay, got added=%d skipped=%d", added, skipped)
	}
	cs, err = st.ListContacts(sb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 {
		t.Fatalf("expected 3 committed contacts after replay, got %d", len(cs))
	}
}

// TestImportBatchIdempotentReplay 验证成功批次重放走指纹幂等：
// 再次导入相同批次应 added=0、skipped=3，不产生重复写入。
func TestImportBatchIdempotentReplay(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/contact_replay.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "B-2", "Batch2", "offshore")
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
	inputs := []ImportInput{
		{FromUnitID: a.ID, ToUnitID: b.ID, Relation: model.RelOverlies, SurveySource: "survey", SurveySeq: 1},
		{FromUnitID: a.ID, ToUnitID: b.ID, Relation: model.RelCuts, SurveySource: "survey", SurveySeq: 2},
		{FromUnitID: b.ID, ToUnitID: a.ID, Relation: model.RelOverlies, SurveySource: "survey", SurveySeq: 3},
	}
	if _, _, err := ImportBatch(st, sb.ID, inputs); err != nil {
		t.Fatalf("first import: %v", err)
	}
	added, skipped, err := ImportBatch(st, sb.ID, inputs)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if added != 0 || skipped != 3 {
		t.Fatalf("expected 0 added/3 skipped on replay, got added=%d skipped=%d", added, skipped)
	}
	cs, err := st.ListContacts(sb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 3 {
		t.Fatalf("expected still 3 contacts after idempotent replay, got %d", len(cs))
	}
}
