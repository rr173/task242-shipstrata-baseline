package strata

import (
	"testing"

	"task242-shipstrata/internal/model"
)

func TestSolveDetectsCycleAndSuspect(t *testing.T) {
	contacts := []model.Contact{
		{ID: "c1", FromUnitID: "a", ToUnitID: "b", Relation: model.RelOverlies, Status: model.ContactStatusConfirmed, SurveySeq: 1},
		{ID: "c2", FromUnitID: "b", ToUnitID: "c", Relation: model.RelCuts, Status: model.ContactStatusConfirmed, SurveySeq: 2},
		{ID: "c3", FromUnitID: "c", ToUnitID: "a", Relation: model.RelOverlies, Status: model.ContactStatusConfirmed, SurveySeq: 3},
	}
	got := Solve(contacts)
	if len(got.Contradictions) != 1 || got.IsConsistent() {
		t.Fatalf("expected one contradiction, got %+v", got)
	}
	if got.Intrusions[0].UnitID == "" {
		t.Fatal("expected a suspect unit")
	}
}

func TestSolveAcyclicGraph(t *testing.T) {
	got := Solve([]model.Contact{{ID: "c1", FromUnitID: "new", ToUnitID: "old", Relation: model.RelOverlies, Status: model.ContactStatusConfirmed}})
	if len(got.Contradictions) != 0 {
		t.Fatalf("acyclic graph reported contradiction: %+v", got)
	}
	if got.Status["c1"] != model.RelOverlies {
		t.Fatalf("unexpected status map: %+v", got.Status)
	}
}

func (r SolveResult) IsConsistent() bool { return len(r.Contradictions) == 0 }

// TestSolvePendingContactsDoNotFormCycle 验证刚导入、尚未确认的待复核测绘
// 关系不参与循环判断：三条本会构成环的 pending 接触不应让图出现矛盾。
// 仅在确认后才参与偏序求解并检出循环。
func TestSolvePendingContactsDoNotFormCycle(t *testing.T) {
	contacts := []model.Contact{
		{ID: "p1", FromUnitID: "a", ToUnitID: "b", Relation: model.RelOverlies, Status: model.ContactStatusPending, SurveySeq: 1},
		{ID: "p2", FromUnitID: "b", ToUnitID: "c", Relation: model.RelCuts, Status: model.ContactStatusPending, SurveySeq: 2},
		{ID: "p3", FromUnitID: "c", ToUnitID: "a", Relation: model.RelOverlies, Status: model.ContactStatusPending, SurveySeq: 3},
	}
	got := Solve(contacts)
	if !got.IsConsistent() || len(got.Contradictions) != 0 {
		t.Fatalf("pending contacts must not form a cycle: %+v", got)
	}
	if got.Status["p1"] != "" || got.Status["p2"] != "" || got.Status["p3"] != "" {
		t.Fatalf("pending contacts must not be projected into status: %+v", got.Status)
	}

	// 确认后三条接触构成环，应检出矛盾并标记 conflict。
	confirmed := []model.Contact{
		{ID: "p1", FromUnitID: "a", ToUnitID: "b", Relation: model.RelOverlies, Status: model.ContactStatusConfirmed, SurveySeq: 1},
		{ID: "p2", FromUnitID: "b", ToUnitID: "c", Relation: model.RelCuts, Status: model.ContactStatusConfirmed, SurveySeq: 2},
		{ID: "p3", FromUnitID: "c", ToUnitID: "a", Relation: model.RelOverlies, Status: model.ContactStatusConfirmed, SurveySeq: 3},
	}
	got = Solve(confirmed)
	if len(got.Contradictions) != 1 {
		t.Fatalf("confirmed cycle must be detected: %+v", got)
	}
	if got.Status["p1"] != model.ContactStatusConflict {
		t.Fatalf("confirmed contact in cycle must be marked conflict: %+v", got.Status)
	}
}
