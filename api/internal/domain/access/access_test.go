package access

import (
	"slices"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func TestGrantEntitlementIsIdempotent(t *testing.T) {
	grantedAt := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	var existing []Entitlement

	first := GrantEntitlement(existing, shared.ID("u1"), shared.ID("m1"), grantedAt)
	if len(first) != 1 {
		t.Fatalf("first grant produced %d entitlements, want 1", len(first))
	}

	second := GrantEntitlement(first, shared.ID("u1"), shared.ID("m1"), grantedAt.Add(time.Hour))
	if len(second) != 1 {
		t.Fatalf("duplicate grant produced %d entitlements, want 1 (idempotent)", len(second))
	}
	if !second[0].GrantedAt.Equal(grantedAt) {
		t.Fatalf("duplicate grant overwrote GrantedAt, got %v", second[0].GrantedAt)
	}
	if !slices.Equal(first, second) {
		t.Fatal("re-granting must yield the identical slice")
	}
}

func TestGrantEntitlementDistinctPairs(t *testing.T) {
	grantedAt := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	got := GrantEntitlement(nil, shared.ID("u1"), shared.ID("m1"), grantedAt)
	got = GrantEntitlement(got, shared.ID("u1"), shared.ID("m2"), grantedAt)
	got = GrantEntitlement(got, shared.ID("u2"), shared.ID("m1"), grantedAt)
	if len(got) != 3 {
		t.Fatalf("distinct pairs produced %d entitlements, want 3", len(got))
	}
}

func TestHasEntitlement(t *testing.T) {
	grantedAt := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	list := GrantEntitlement(nil, shared.ID("u1"), shared.ID("m1"), grantedAt)
	if !HasEntitlement(list, shared.ID("u1"), shared.ID("m1")) {
		t.Fatal("granted pair must be found")
	}
	if HasEntitlement(list, shared.ID("u1"), shared.ID("m2")) {
		t.Fatal("different product must not be found")
	}
	if HasEntitlement(list, shared.ID("u2"), shared.ID("m1")) {
		t.Fatal("different user must not be found")
	}
	if HasEntitlement(nil, shared.ID("u1"), shared.ID("m1")) {
		t.Fatal("empty list must not contain anything")
	}
}

func TestGrantEntitlementDoesNotMutateInput(t *testing.T) {
	grantedAt := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	original := GrantEntitlement(nil, shared.ID("u1"), shared.ID("m1"), grantedAt)
	snapshot := slices.Clone(original)
	GrantEntitlement(original, shared.ID("u1"), shared.ID("m2"), grantedAt)
	if !slices.Equal(original, snapshot) {
		t.Fatal("grant mutated its input slice")
	}
}
