package app

import (
	"context"
	"errors"
	"testing"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func mustProduct(t *testing.T, id, slug string) catalog.Product {
	t.Helper()
	p, err := catalog.NewProduct(shared.ID(id), slug, "Material", shared.NewMoney(4700), catalog.KindMaterial, "files/"+slug+".pdf", 0, 0)
	if err != nil {
		t.Fatalf("NewProduct: %v", err)
	}
	return p
}

func TestListProductsHidesDeactivated(t *testing.T) {
	ctx := context.Background()
	repo := &fakes.ProductRepo{}
	uc := NewCatalog(repo)

	live := mustProduct(t, "p1", "guia-de-pronuncia")
	retired := mustProduct(t, "p2", "curso-antigo").Deactivate()
	for _, p := range []catalog.Product{live, retired} {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	got, err := uc.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d products, want only the active one", len(got))
	}
	if got[0].Slug != "guia-de-pronuncia" {
		t.Errorf("slug = %q, want the active product", got[0].Slug)
	}
}

// An old link to a retired product should still render a page; refusing the
// sale is checkout's job, not the catalogue's.
func TestGetProductReturnsDeactivated(t *testing.T) {
	ctx := context.Background()
	repo := &fakes.ProductRepo{}
	uc := NewCatalog(repo)

	if err := repo.Create(ctx, mustProduct(t, "p2", "curso-antigo").Deactivate()); err != nil {
		t.Fatal(err)
	}

	got, err := uc.GetProduct(ctx, "curso-antigo")
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.Active {
		t.Error("expected the retired product, got an active one")
	}
}

func TestGetProductUnknownSlugIsNotFound(t *testing.T) {
	uc := NewCatalog(&fakes.ProductRepo{})
	if _, err := uc.GetProduct(context.Background(), "nao-existe"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("err = %v, want ports.ErrNotFound", err)
	}
}
