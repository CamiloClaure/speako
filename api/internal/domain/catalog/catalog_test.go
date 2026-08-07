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
			// A new product is on sale by default; retiring it is explicit.
			p.Active = true
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

// A teacher who cannot tell which field is wrong cannot fix her form, so
// NewProduct must return a sentinel distinguishable by field, not just the
// generic ErrInvalidProduct family.
func TestNewProductRejectsNamesTheField(t *testing.T) {
	tests := []struct {
		name string
		p    Product
		want error
	}{
		{"missing id", Product{Slug: "s", Title: "T", Price: shared.NewMoney(100), Kind: KindMaterial, FileKey: "k"}, ErrProductMissingID},
		{"missing slug", Product{ID: "p1", Title: "T", Price: shared.NewMoney(100), Kind: KindMaterial, FileKey: "k"}, ErrProductMissingSlug},
		{"invalid price", Product{ID: "p1", Slug: "s", Title: "T", Price: shared.NewMoney(0), Kind: KindMaterial, FileKey: "k"}, ErrProductInvalidPrice},
		{"missing file key", Product{ID: "p1", Slug: "s", Title: "T", Price: shared.NewMoney(100), Kind: KindMaterial}, ErrProductMissingFileKey},
		{"invalid credits", Product{ID: "p1", Slug: "s", Title: "T", Price: shared.NewMoney(100), Kind: KindPack}, ErrProductInvalidCredits},
		{"invalid kind", Product{ID: "p1", Slug: "s", Title: "T", Price: shared.NewMoney(100), Kind: Kind("subscription")}, ErrProductInvalidKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProduct(tt.p.ID, tt.p.Slug, tt.p.Title, tt.p.Price, tt.p.Kind, tt.p.FileKey, tt.p.Credits, tt.p.ValidityDays)
			if !errors.Is(err, tt.want) {
				t.Fatalf("NewProduct error = %v, want %v", err, tt.want)
			}
			if !errors.Is(err, ErrInvalidProduct) {
				t.Fatalf("NewProduct error %v does not also satisfy the ErrInvalidProduct family", err)
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

func TestSlugify(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		want    string
		wantErr bool
	}{
		{"simple", "Ebook Iniciante", "ebook-iniciante", false},
		{"pt-BR accents and em dash", "Pacote de 8 Aulas — Conversação", "pacote-de-8-aulas-conversacao", false},
		{"cedilla and tilde", "Lição de Conversação com Situações do Dia a Dia", "licao-de-conversacao-com-situacoes-do-dia-a-dia", false},
		{"leading and trailing punctuation", "  -- Guia Prático! --  ", "guia-pratico", false},
		{"collapses repeated separators", "Pacote   4   Aulas///Premium", "pacote-4-aulas-premium", false},
		{"all punctuation fails instead of producing an empty slug", "--- !!! ---", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Slugify(tt.title)
			if tt.wantErr {
				if !errors.Is(err, ErrEmptySlug) {
					t.Fatalf("Slugify(%q) err = %v, want ErrEmptySlug", tt.title, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Slugify(%q): %v", tt.title, err)
			}
			if got != tt.want {
				t.Fatalf("Slugify(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestActivateDeactivate(t *testing.T) {
	p, err := NewProduct("p1", "guia", "Guia", shared.NewMoney(4700), KindMaterial, "files/guia.pdf", 0, 0)
	if err != nil {
		t.Fatalf("NewProduct: %v", err)
	}
	if !p.Active {
		t.Fatal("a new product must be on sale")
	}
	retired := p.Deactivate()
	if retired.Active {
		t.Error("Deactivate left the product on sale")
	}
	if !p.Active {
		t.Error("Deactivate mutated the receiver")
	}
	if !retired.Activate().Active {
		t.Error("Activate did not put it back on sale")
	}
}
