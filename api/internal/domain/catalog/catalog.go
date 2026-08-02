package catalog

import (
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrInvalidProduct is returned when a product violates any construction rule:
// empty slug, non-positive price, a pack without credits, or a material
// without a file key.
var ErrInvalidProduct = shared.NewDomainError("product.invalid", "invalid product")

// Kind discriminates the two things the shop sells.
type Kind string

const (
	// KindMaterial is a digital file (PDF/ebook). It carries a file key.
	KindMaterial Kind = "material"
	// KindPack is N lesson credits with a validity period.
	KindPack Kind = "pack"
)

func (k Kind) IsMaterial() bool { return k == KindMaterial }
func (k Kind) IsPack() bool     { return k == KindPack }

// Product is a sellable item. Exactly one side of the payload is populated,
// depending on Kind: FileKey for a material, Credits and ValidityDays for a
// pack. ValidityDays zero means the credits never expire.
type Product struct {
	ID           shared.ID
	Slug         string
	Title        string
	Price        shared.Money
	Kind         Kind
	FileKey      string
	Credits      int
	ValidityDays int
}

// NewProduct validates the shape of a product and returns it as a value.
func NewProduct(id shared.ID, slug, title string, price shared.Money, kind Kind, fileKey string, credits, validityDays int) (Product, error) {
	if id.IsZero() {
		return Product{}, ErrInvalidProduct
	}
	if slug == "" {
		return Product{}, ErrInvalidProduct
	}
	if price.Cents() <= 0 {
		return Product{}, ErrInvalidProduct
	}
	switch kind {
	case KindMaterial:
		if fileKey == "" {
			return Product{}, ErrInvalidProduct
		}
	case KindPack:
		if credits < 1 {
			return Product{}, ErrInvalidProduct
		}
	default:
		return Product{}, ErrInvalidProduct
	}
	return Product{
		ID:           id,
		Slug:         slug,
		Title:        title,
		Price:        price,
		Kind:         kind,
		FileKey:      fileKey,
		Credits:      credits,
		ValidityDays: validityDays,
	}, nil
}
