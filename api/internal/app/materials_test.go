package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/domain/access"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

type recordingFileStorage struct {
	files *fakes.FileStorage
	calls int
}

func (s *recordingFileStorage) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	s.calls++
	return s.files.SignedURL(ctx, key, ttl)
}

type materialsHarness struct {
	uc           *Materials
	products     *fakes.ProductRepo
	entitlements *fakes.EntitlementRepo
	files        *recordingFileStorage
}

func newMaterialsHarness(t *testing.T) materialsHarness {
	t.Helper()
	h := materialsHarness{
		products:     &fakes.ProductRepo{},
		entitlements: &fakes.EntitlementRepo{},
		files:        &recordingFileStorage{files: &fakes.FileStorage{}},
	}
	h.uc = NewMaterials(h.products, h.entitlements, h.files)
	return h
}

func TestListMyMaterialsOnlyMaterials(t *testing.T) {
	h := newMaterialsHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")

	ebook, err := catalog.NewProduct(shared.ID("prod-ebook"), "ebook-a", "Ebook A", shared.NewMoney(4700), catalog.KindMaterial, "materials/ebook-a.pdf", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := catalog.NewProduct(shared.ID("prod-pack"), "pack-4", "Pacote 4 aulas", shared.NewMoney(30000), catalog.KindPack, "", 4, 90)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.products.Create(ctx, ebook); err != nil {
		t.Fatal(err)
	}
	if err := h.products.Create(ctx, pack); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)
	if err := h.entitlements.Add(ctx, access.Entitlement{UserID: userID, ProductID: ebook.ID, GrantedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := h.entitlements.Add(ctx, access.Entitlement{UserID: userID, ProductID: pack.ID, GrantedAt: now}); err != nil {
		t.Fatal(err)
	}

	got, err := h.uc.ListMyMaterials(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != ebook.ID {
		t.Fatalf("got %+v, want just the ebook", got)
	}
}

func TestIssueDownloadURLWithoutEntitlement(t *testing.T) {
	h := newMaterialsHarness(t)
	ctx := context.Background()
	ebook, err := catalog.NewProduct(shared.ID("prod-ebook"), "ebook-a", "Ebook A", shared.NewMoney(4700), catalog.KindMaterial, "materials/ebook-a.pdf", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.products.Create(ctx, ebook); err != nil {
		t.Fatal(err)
	}

	_, err = h.uc.IssueDownloadURL(ctx, shared.ID("user-1"), ebook.Slug)
	if !errors.Is(err, ErrNoEntitlement) {
		t.Fatalf("err = %v, want ErrNoEntitlement", err)
	}
	if h.files.calls != 0 {
		t.Fatalf("FileStorage was called %d times without an entitlement", h.files.calls)
	}
}

func TestIssueDownloadURLWithEntitlement(t *testing.T) {
	h := newMaterialsHarness(t)
	ctx := context.Background()
	userID := shared.ID("user-1")
	ebook, err := catalog.NewProduct(shared.ID("prod-ebook"), "ebook-a", "Ebook A", shared.NewMoney(4700), catalog.KindMaterial, "materials/ebook-a.pdf", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.products.Create(ctx, ebook); err != nil {
		t.Fatal(err)
	}
	if err := h.entitlements.Add(ctx, access.Entitlement{UserID: userID, ProductID: ebook.ID, GrantedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	url, err := h.uc.IssueDownloadURL(ctx, userID, ebook.Slug)
	if err != nil {
		t.Fatalf("IssueDownloadURL: %v", err)
	}
	want := "https://cdn.test/" + ebook.FileKey
	if url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
	if h.files.calls != 1 {
		t.Fatalf("FileStorage called %d times, want 1", h.files.calls)
	}
}

func TestIssueDownloadURLRejectsPack(t *testing.T) {
	h := newMaterialsHarness(t)
	ctx := context.Background()
	pack, err := catalog.NewProduct(shared.ID("prod-pack"), "pack-4", "Pacote 4 aulas", shared.NewMoney(30000), catalog.KindPack, "", 4, 90)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.products.Create(ctx, pack); err != nil {
		t.Fatal(err)
	}

	_, err = h.uc.IssueDownloadURL(ctx, shared.ID("user-1"), pack.Slug)
	if !errors.Is(err, ErrNoEntitlement) {
		t.Fatalf("err = %v, want ErrNoEntitlement", err)
	}
	if h.files.calls != 0 {
		t.Fatalf("FileStorage was called %d times for a pack", h.files.calls)
	}
}
