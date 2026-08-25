package strata_test

import (
	"context"
	"testing"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/store"
	"task242-shipstrata/internal/contact"
	"task242-shipstrata/internal/site"
)

// TestReconcileClearsStaleConflictAfterReject is the regression for the
// reported defect: a triangular set of contacts forms a cycle (all marked
// conflict); after rejecting one edge and re-running reconcile, the still-
// valid relations must be restored to participating state and the stale
// conflict marker must be cleared, with the graph reporting consistent.
func TestReconcileClearsStaleConflictAfterReject(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenStore(t.TempDir() + "/regress.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	svc := service.New(st)

	sb, err := site.Create(st, "R-1", "Regress Site", "offshore")
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	uA, _ := site.CreateUnit(st, sb.ID, "A", "sediment", 0, 1, "")
	uB, _ := site.CreateUnit(st, sb.ID, "B", "sediment", 1, 2, "")
	uC, _ := site.CreateUnit(st, sb.ID, "C", "sediment", 2, 3, "")

	_, c1, _ := contact.Import(st, sb.ID, uA.ID, uB.ID, model.RelOverlies, "survey1", 1, "")
	_, c2, _ := contact.Import(st, sb.ID, uB.ID, uC.ID, model.RelOverlies, "survey1", 2, "")
	_, c3, _ := contact.Import(st, sb.ID, uC.ID, uA.ID, model.RelOverlies, "survey1", 3, "")
	for _, id := range []string{c1.ID, c2.ID, c3.ID} {
		if _, err := contact.Confirm(st, id); err != nil {
			t.Fatalf("confirm %s: %v", id, err)
		}
	}

	// Triangular cycle -> all three marked conflict, one contradiction.
	if _, err := svc.Strata.Reconcile(ctx, sb.ID); err != nil {
		t.Fatalf("reconcile cycle: %v", err)
	}
	cds, _ := st.ListContradictions(sb.ID)
	if openCount(t, cds) != 1 {
		t.Fatalf("expected 1 open contradiction, got %d", openCount(t, cds))
	}

	// Reject one edge to break the cycle, then re-reconcile.
	if _, err := contact.Reject(st, c1.ID); err != nil {
		t.Fatalf("reject c1: %v", err)
	}
	if _, err := svc.Strata.Reconcile(ctx, sb.ID); err != nil {
		t.Fatalf("reconcile after reject: %v", err)
	}

	// Still-valid relations (c2, c3) must NOT remain conflict.
	cls, err := st.ListContacts(sb.ID)
	if err != nil {
		t.Fatalf("list contacts: %v", err)
	}
	for _, c := range cls {
		if c.ID == c1.ID {
			if c.Status != model.ContactStatusExcluded {
				t.Errorf("rejected contact %s should be excluded, got %s", c.ID, c.Status)
			}
			continue
		}
		if c.Status == model.ContactStatusConflict {
			t.Errorf("still-valid contact %s remains marked conflict after cycle broken", c.ID)
		}
		if c.Status == model.ContactStatusPending {
			t.Errorf("still-valid contact %s restored to pending instead of confirmed, dropped from partial order", c.ID)
		}
	}

	// Stale contradiction marker must be cleared (resolved).
	cds2, _ := st.ListContradictions(sb.ID)
	if open := openCount(t, cds2); open != 0 {
		t.Errorf("expected 0 open contradictions after cycle broken, got %d", open)
	}

	// Graph must report a consistent partial order.
	v, err := svc.Strata.Solve(ctx, sb.ID)
	if err != nil {
		t.Fatalf("solve: %v", err)
	}
	if !v.IsConsistent {
		t.Errorf("expected consistent graph after cycle broken, got cycles=%v", v.Cycles)
	}
}

func openCount(t *testing.T, cds []model.Contradiction) int {
	t.Helper()
	n := 0
	for _, c := range cds {
		if !c.Resolved {
			n++
		}
	}
	return n
}
