package catalog

import (
	"errors"
	"testing"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func TestNewProductValid(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		p    Product
	}{
		{
			"material", KindMaterial,
			Product{ID: "p1", Slug: "ebook-iniciante", Title: "E-book Iniciante", Price: shared.NewMoney(2990), Kind: KindMaterial, FileKey: "materials/ebook-iniciante.pdf"},
		},
		{
			"pack", KindPack,
			Product{ID: "p2", Slug: "pacote-4-aulas", Title: "Pacote 4 aulas", Price: shared.NewMoney(39990), Kind: KindPack, Credits: 4, ValidityDays: 90},
		},
		{
			"pack without expiry", KindPack,
			Product{ID: "p3", Slug: "pacote-1-aula", Title: "Pacote 1 aula", Price: shared.NewMoney(11990), Kind: KindPack, Credits: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.p
			got, err := NewProduct(p.ID, p.Slug, p.Title, p.Price, p.Kind, p.FileKey, p.Credits, p.ValidityDays)
			if err != nil {
				t.Fatalf("NewProduct: %v", err)
			}
			if got != p {
				t.Fatalf("NewProduct = %+v, want %+v", got, p)
			}
		})
	}
}

func TestNewProductRejects(t *testing.T) {
	validMaterial := Product{ID: "p1", Slug: "ebook", Title: "E-book", Price: shared.NewMoney(1000), Kind: KindMaterial, FileKey: "k"}
	validPack := Product{ID: "p2", Slug: "pack", Title: "Pack", Price: shared.NewMoney(2000), Kind: KindPack, Credits: 2}

	tests := []struct {
		name   string
		mutate func(*Product)
	}{
		{"empty slug", func(p *Product) { p.Slug = "" }},
		{"zero price", func(p *Product) { p.Price = shared.NewMoney(0) }},
		{"negative price", func(p *Product) { p.Price = shared.NewMoney(-1) }},
		{"material without file key", func(p *Product) { p.FileKey = "" }},
		{"pack without credits", func(p *Product) { p.Credits = 0 }},
		{"pack with negative credits", func(p *Product) { p.Credits = -1 }},
		{"unknown kind", func(p *Product) { p.Kind = Kind("subscription") }},
		{"empty id", func(p *Product) { p.ID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := validMaterial
			if tt.name == "pack without credits" || tt.name == "pack with negative credits" {
				base = validPack
			}
			p := base
			tt.mutate(&p)
			_, err := NewProduct(p.ID, p.Slug, p.Title, p.Price, p.Kind, p.FileKey, p.Credits, p.ValidityDays)
			if !errors.Is(err, ErrInvalidProduct) {
				t.Fatalf("NewProduct error = %v, want %v", err, ErrInvalidProduct)
			}
		})
	}
}

func TestKindPredicates(t *testing.T) {
	if !KindMaterial.IsMaterial() || KindMaterial.IsPack() {
		t.Fatal("KindMaterial must be material and not pack")
	}
	if !KindPack.IsPack() || KindPack.IsMaterial() {
		t.Fatal("KindPack must be pack and not material")
	}
}
