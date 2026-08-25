package model

import "testing"

func TestValidationHelpers(t *testing.T) {
	for _, rel := range []string{RelOverlies, RelCuts} {
		if !ValidRelation(rel) {
			t.Fatalf("relation %q should be valid", rel)
		}
	}
	if ValidRelation("touches") {
		t.Fatal("unknown relation should be rejected")
	}
	for _, status := range []string{SiteStatusCollecting, SiteStatusPendingReview, SiteStatusPublished, SiteStatusSealed} {
		if !ValidSiteStatus(status) {
			t.Fatalf("site status %q should be valid", status)
		}
	}
}

func TestNewIDHasPrefixAndEntropy(t *testing.T) {
	a := NewID("unit")
	b := NewID("unit")
	if len(a) <= len("unit_") || a[:len("unit_")] != "unit_" {
		t.Fatalf("unexpected id %q", a)
	}
	if a == b {
		t.Fatalf("two generated ids must differ: %q", a)
	}
}

func TestContactParticipatesOnlyAfterConfirmation(t *testing.T) {
	cases := map[string]bool{
		ContactStatusPending:   false, // 刚导入、尚未确认的待复核关系不参与偏序
		ContactStatusExcluded:  false, // 否决误连后排除
		ContactStatusConfirmed: true,  // 研究者确认后参与
		ContactStatusConflict:  true,  // 已确认边派生的矛盾标记仍参与（保留环信息）
	}
	for status, want := range cases {
		if got := ContactParticipates(status); got != want {
			t.Fatalf("ContactParticipates(%q) = %v, want %v", status, got, want)
		}
	}
}
