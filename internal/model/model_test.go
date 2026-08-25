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
