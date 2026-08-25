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
