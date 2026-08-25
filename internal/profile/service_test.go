package profile

import (
	"testing"

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
