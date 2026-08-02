package app

import (
	"context"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
)

// Catalog reads the public shop: the materials and lesson packs on sale.
type Catalog struct {
	products ports.ProductRepo
}

// NewCatalog wires the catalogue use cases to their ports.
func NewCatalog(products ports.ProductRepo) *Catalog {
	return &Catalog{products: products}
}

// ListProducts returns only what is on sale. Deactivating a product must hide
// it from the shop without deleting it, because past orders still reference it.
func (uc *Catalog) ListProducts(ctx context.Context) ([]catalog.Product, error) {
	all, err := uc.products.List(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]catalog.Product, 0, len(all))
	for _, p := range all {
		if p.Active {
			active = append(active, p)
		}
	}
	return active, nil
}

// GetProduct returns a single product by slug, active or not: a buyer who
// follows an old link should see the page, and the checkout is what refuses.
func (uc *Catalog) GetProduct(ctx context.Context, slug string) (catalog.Product, error) {
	return uc.products.GetBySlug(ctx, slug)
}
