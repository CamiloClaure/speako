package catalog

import (
	"strings"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrInvalidProduct is the family sentinel for every product construction
// failure. Match it with errors.Is for "was this a product problem at all";
// the specific errors below share its kind (see shared.DomainError.Is) so
// they also satisfy errors.Is(err, ErrInvalidProduct), while carrying a
// message that names the one field that was wrong.
var (
	ErrInvalidProduct        = shared.NewDomainError("product.invalid", "invalid product")
	ErrProductMissingID      = shared.NewDomainError("product.invalid", "id is required")
	ErrProductMissingSlug    = shared.NewDomainError("product.invalid", "slug is required")
	ErrProductInvalidPrice   = shared.NewDomainError("product.invalid", "price_cents must be greater than zero")
	ErrProductMissingFileKey = shared.NewDomainError("product.invalid", "file_key is required when kind is material")
	ErrProductInvalidCredits = shared.NewDomainError("product.invalid", "credits must be at least 1 when kind is pack")
	ErrProductInvalidKind    = shared.NewDomainError("product.invalid", "kind must be material or pack")
	// ErrEmptySlug is returned by Slugify when a title collapses to nothing
	// (e.g. it is all punctuation). It shares ErrInvalidProduct's kind so it
	// maps to the same 422 validation_failed response without touching
	// internal/http/errors.go.
	ErrEmptySlug = shared.NewDomainError("product.invalid", "title must contain at least one letter or number to derive a slug")
	// ErrSlugTaken is returned when a different product already owns the slug
	// this title derives to. The slug is the product's public URL, so two
	// products cannot share one.
	ErrSlugTaken = shared.NewDomainError("product.invalid", "another product already uses this title")
)

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
// Active controls whether the product appears in the shop. Products are
// deactivated rather than deleted, because past orders still reference them.
type Product struct {
	ID           shared.ID
	Slug         string
	Title        string
	Price        shared.Money
	Kind         Kind
	FileKey      string
	Credits      int
	ValidityDays int
	Active       bool
}

// NewProduct validates the shape of a product and returns it as a value.
func NewProduct(id shared.ID, slug, title string, price shared.Money, kind Kind, fileKey string, credits, validityDays int) (Product, error) {
	if id.IsZero() {
		return Product{}, ErrProductMissingID
	}
	if slug == "" {
		return Product{}, ErrProductMissingSlug
	}
	if price.Cents() <= 0 {
		return Product{}, ErrProductInvalidPrice
	}
	switch kind {
	case KindMaterial:
		if fileKey == "" {
			return Product{}, ErrProductMissingFileKey
		}
	case KindPack:
		if credits < 1 {
			return Product{}, ErrProductInvalidCredits
		}
	default:
		return Product{}, ErrProductInvalidKind
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
		Active:       true,
	}, nil
}

// Deactivate takes a product off sale without deleting it.
func (p Product) Deactivate() Product {
	p.Active = false
	return p
}

// Activate puts a product back on sale.
func (p Product) Activate() Product {
	p.Active = true
	return p
}

// accentFold maps the lowercase Latin letters a pt-BR title actually uses to
// their unaccented ASCII equivalent. Slugify lowercases first, so uppercase
// accented runes never reach this table.
//
// ponytail: a hand-written ~15-rune table instead of golang.org/x/text/unicode/norm
// - this repo has three direct dependencies and pt-BR titles use a small,
// known alphabet. Extend the table if a title ever needs a rune outside it.
var accentFold = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i',
	'ó': 'o', 'ò': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u',
	'ç': 'c', 'ñ': 'n', 'ý': 'y',
}

// Slugify derives a URL-safe slug from a product title: lowercased, accents
// folded to ASCII, every run of non-alphanumeric characters collapsed to one
// hyphen, and the result trimmed of leading/trailing hyphens. It returns
// ErrEmptySlug rather than an empty string when the title has no letters or
// digits at all (e.g. it is only punctuation) - an empty slug is not a valid
// product identifier, so that has to fail like any other invalid field.
func Slugify(title string) (string, error) {
	var b strings.Builder
	lastWasHyphen := true // starts true so a leading run of punctuation is dropped, not hyphenated
	for _, r := range strings.ToLower(title) {
		if folded, ok := accentFold[r]; ok {
			r = folded
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasHyphen = false
		case !lastWasHyphen:
			b.WriteByte('-')
			lastWasHyphen = true
		}
	}
	slug := strings.TrimSuffix(b.String(), "-")
	if slug == "" {
		return "", ErrEmptySlug
	}
	return slug, nil
}
