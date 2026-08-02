package access

import (
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// Entitlement is the fact that a user may open a material. It is granted by
// purchasing the material and revoked only by refunding that order.
type Entitlement struct {
	UserID    shared.ID
	ProductID shared.ID
	GrantedAt time.Time
}

// HasEntitlement reports whether the list already grants this pair.
func HasEntitlement(existing []Entitlement, userID, productID shared.ID) bool {
	for _, e := range existing {
		if e.UserID == userID && e.ProductID == productID {
			return true
		}
	}
	return false
}

// GrantEntitlement appends the entitlement unless it is already present, so
// granting twice for the same (user, product) yields the same single
// entitlement. It never fails and never mutates the input slice.
func GrantEntitlement(existing []Entitlement, userID, productID shared.ID, grantedAt time.Time) []Entitlement {
	if HasEntitlement(existing, userID, productID) {
		return existing
	}
	out := append([]Entitlement(nil), existing...)
	return append(out, Entitlement{
		UserID:    userID,
		ProductID: productID,
		GrantedAt: grantedAt,
	})
}
