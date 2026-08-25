package profile

import (
	"sort"
	"sync"
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

// TestPublishConcurrentVersions 验证同一遗址并发发布 20 个剖面版本时，
// 每个成功版本的版本号都唯一且连续（1..N）。修复前两步式分配
// （NextProfileVersion + CreateProfile）会让并发发布者读到同一个 MAX(version)
// 从而写入重复版本号。
func TestPublishConcurrentVersions(t *testing.T) {
	st, err := store.OpenStore(t.TempDir() + "/profile_concurrent.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	sb, err := site.Create(st, "C-1", "Concurrent", "offshore")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, strata.NewService(st))

	const n = 20
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		versions []int
		errs    []error
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			p, _, e := svc.Publish(t.Context(), sb.ID, PublishInput{Label: "v"})
			mu.Lock()
			if e != nil {
				errs = append(errs, e)
			} else {
				versions = append(versions, p.Version)
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(errs) != 0 {
		t.Fatalf("publish failures: %v", errs)
	}
	if len(versions) != n {
		t.Fatalf("expected %d published versions, got %d", n, len(versions))
	}
	sort.Ints(versions)
	for i, v := range versions {
		if v != i+1 {
			t.Fatalf("versions not unique/continuous: got %v", versions)
		}
	}

	// 库内版本号应与返回值一致且唯一。
	stored, err := svc.List(t.Context(), sb.ID)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[int]bool, len(stored))
	for _, p := range stored {
		if seen[p.Version] {
			t.Fatalf("duplicate version %d persisted", p.Version)
		}
		seen[p.Version] = true
	}
	if len(seen) != n {
		t.Fatalf("expected %d distinct stored versions, got %d", n, len(seen))
	}
}
