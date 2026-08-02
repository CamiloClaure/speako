package app

import (
	"context"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/access"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

var (
	// ErrNoEntitlement is returned when someone without access asks for a
	// material download URL.
	ErrNoEntitlement = shared.NewDomainError("entitlement.required", "no entitlement for this material")
)

// downloadURLTTL is how long a signed material URL stays valid.
// A download link is short-lived on purpose: it is a bearer token for a paid
// file, and anything longer is a shareable link.
const downloadURLTTL = 5 * time.Minute

// Materials hands out what a user has actually bought: the materials they are
// entitled to, and short-lived download links that are only minted after the
// entitlement check passes.
type Materials struct {
	products     ports.ProductRepo
	entitlements ports.EntitlementRepo
	files        ports.FileStorage
}

// NewMaterials wires the material delivery use cases to their ports.
func NewMaterials(products ports.ProductRepo, entitlements ports.EntitlementRepo, files ports.FileStorage) *Materials {
	return &Materials{
		products:     products,
		entitlements: entitlements,
		files:        files,
	}
}

// ListMyMaterials returns every material the user is entitled to open. Packs
// do not grant material access, so they never appear here.
func (uc *Materials) ListMyMaterials(ctx context.Context, userID shared.ID) ([]catalog.Product, error) {
	entitlements, err := uc.entitlements.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []catalog.Product
	for _, e := range entitlements {
		p, err := uc.products.GetByID(ctx, e.ProductID)
		if err != nil {
			return nil, err
		}
		if p.Kind.IsMaterial() {
			out = append(out, p)
		}
	}
	return out, nil
}

// IssueDownloadURL mints a temporary download URL, but only for a user the
// entitlement check admits. No entitlement means FileStorage is never asked.
func (uc *Materials) IssueDownloadURL(ctx context.Context, userID shared.ID, productSlug string) (string, error) {
	product, err := uc.products.GetBySlug(ctx, productSlug)
	if err != nil {
		return "", err
	}
	if !product.Kind.IsMaterial() {
		return "", ErrNoEntitlement
	}
	entitlements, err := uc.entitlements.ListByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if !access.HasEntitlement(entitlements, userID, product.ID) {
		return "", ErrNoEntitlement
	}
	return uc.files.SignedURL(ctx, product.FileKey, downloadURLTTL)
}
