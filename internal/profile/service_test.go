package profile

import (
	"errors"
	"testing"

	"task242-shipstrata/internal/model"
	"task242-shipstrata/internal/site"
	"task242-shipstrata/internal/store"
	"task242-shipstrata/internal/strata"
)

func TestPublishShareFreezeRoundTrip(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/profile.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "P-1", "Profiles", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, strata.NewService(st))
	p, _, err := svc.Publish(t.Context(), sb.ID, PublishInput{Label: "v1"})
	if err != nil || p.Version != 1 {
		t.Fatalf("publish failed: %+v err=%v", p, err)
	}
	if _, err := svc.Share(t.Context(), p.ID); err != nil {
		t.Fatal(err)
	}
	frozen, err := svc.Freeze(t.Context(), p.ID)
	if err != nil || frozen.Status != "frozen" || frozen.FrozenAt == nil {
		t.Fatalf("freeze failed: %+v err=%v", frozen, err)
	}
	got, snap, err := svc.Get(t.Context(), p.ID)
	if err != nil || got.Snapshot == "" || snap.GeneratedAt == "" {
		t.Fatalf("snapshot round trip failed: %+v %+v err=%v", got, snap, err)
	}
}

// publishFrozen publishes, shares and freezes a profile on siteID, returning it.
func publishFrozen(t *testing.T, svc *Service, siteID string) model.ProfileVersion {
	t.Helper()
	p, _, err := svc.Publish(t.Context(), siteID, PublishInput{Label: "v1"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := svc.Share(t.Context(), p.ID); err != nil {
		t.Fatalf("share: %v", err)
	}
	frozen, err := svc.Freeze(t.Context(), p.ID)
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	return frozen
}

// TestSupersedeSameSite retires an old profile in favor of a fresh draft within
// the same site, then confirms the old version is marked superseded.
func TestSupersedeSameSite(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/sup.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "S-1", "Same", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, strata.NewService(st))
	old := publishFrozen(t, svc, sb.ID)
	newP, _, err := svc.Publish(t.Context(), sb.ID, PublishInput{Label: "v2"})
	if err != nil {
		t.Fatalf("publish new: %v", err)
	}
	if err := svc.Supersede(t.Context(), old.ID, newP.ID); err != nil {
		t.Fatalf("supersede same site: %v", err)
	}
	got, _, err := svc.Get(t.Context(), old.ID)
	if err != nil || got.Status != model.ProfileStatusSuperseded {
		t.Fatalf("old not superseded: %+v err=%v", got, err)
	}
}

// TestSupersedeRejectsCrossSite verifies that superseding an old profile with a
// draft from a different site is rejected and leaves both statuses untouched.
func TestSupersedeRejectsCrossSite(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/cross.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sbA, err := site.Create(st, "A-1", "Site A", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	sbB, err := site.Create(st, "B-1", "Site B", "inshore")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, strata.NewService(st))
	oldA := publishFrozen(t, svc, sbA.ID)
	newB, _, err := svc.Publish(t.Context(), sbB.ID, PublishInput{Label: "v1"})
	if err != nil {
		t.Fatalf("publish B: %v", err)
	}
	err = svc.Supersede(t.Context(), oldA.ID, newB.ID)
	if !errors.Is(err, model.ErrCrossSite) {
		t.Fatalf("expected ErrCrossSite, got %v", err)
	}
	// Both statuses must be unchanged.
	if got, _, e := svc.Get(t.Context(), oldA.ID); e != nil || got.Status != model.ProfileStatusFrozen {
		t.Fatalf("old status changed: %+v err=%v", got, e)
	}
	if got, _, e := svc.Get(t.Context(), newB.ID); e != nil || got.Status != model.ProfileStatusDraft {
		t.Fatalf("new status changed: %+v err=%v", got, e)
	}
}

// TestSupersedeRejectsIllegalStatus verifies that a supersede whose old/new
// lifecycle is illegal (e.g. retiring a draft) is rejected and changes nothing.
func TestSupersedeRejectsIllegalStatus(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/status.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "S-2", "Status", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, strata.NewService(st))
	oldDraft, _, err := svc.Publish(t.Context(), sb.ID, PublishInput{Label: "d1"})
	if err != nil {
		t.Fatalf("publish old: %v", err)
	}
	newDraft, _, err := svc.Publish(t.Context(), sb.ID, PublishInput{Label: "d2"})
	if err != nil {
		t.Fatalf("publish new: %v", err)
	}
	err = svc.Supersede(t.Context(), oldDraft.ID, newDraft.ID)
	if !errors.Is(err, model.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
	if got, _, e := svc.Get(t.Context(), oldDraft.ID); e != nil || got.Status != model.ProfileStatusDraft {
		t.Fatalf("old status changed: %+v err=%v", got, e)
	}
}
