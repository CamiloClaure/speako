package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/fakes"
	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

type purchaseHarness struct {
	uc           *Purchase
	products     *fakes.ProductRepo
	orders       *fakes.OrderRepo
	users        *fakes.UserRepo
	entitlements *fakes.EntitlementRepo
	ledger       *fakes.CreditLedgerRepo
	events       *fakes.PaymentEventRepo
	payments     *fakes.PaymentGateway
	mailer       *fakes.Mailer
	tx           *fakes.TxManager
	clock        *fakes.Clock
	idgen        *fakes.IDGen
}

func newPurchaseHarness(t *testing.T) purchaseHarness {
	t.Helper()
	h := purchaseHarness{
		products:     &fakes.ProductRepo{},
		orders:       &fakes.OrderRepo{},
		users:        &fakes.UserRepo{},
		entitlements: &fakes.EntitlementRepo{},
		ledger:       &fakes.CreditLedgerRepo{},
		events:       &fakes.PaymentEventRepo{},
		payments:     &fakes.PaymentGateway{},
		mailer:       &fakes.Mailer{},
		tx:           &fakes.TxManager{},
		clock:        &fakes.Clock{NowTime: time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC)},
		idgen:        &fakes.IDGen{},
	}
	h.uc = NewPurchase(h.products, h.orders, h.users, h.entitlements, h.ledger, h.events, h.payments, h.mailer, h.tx, h.clock, h.idgen)
	return h
}

func mustUser(t *testing.T, id, rawEmail string) identity.User {
	t.Helper()
	email, err := shared.ParseEmail(rawEmail)
	if err != nil {
		t.Fatalf("parse email: %v", err)
	}
	u, err := identity.NewUser(shared.ID(id), email, "Ana", identity.RoleStudent, "America/Sao_Paulo")
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	return u
}

func materialProduct(t *testing.T) catalog.Product {
	t.Helper()
	p, err := catalog.NewProduct(shared.ID("prod-material"), "ebook-iniciante", "Ebook Iniciante", shared.NewMoney(4700), catalog.KindMaterial, "materials/ebook-iniciante.pdf", 0, 0)
	if err != nil {
		t.Fatalf("new product: %v", err)
	}
	return p
}

func packProduct(t *testing.T) catalog.Product {
	t.Helper()
	p, err := catalog.NewProduct(shared.ID("prod-pack"), "pack-4", "Pacote 4 aulas", shared.NewMoney(30000), catalog.KindPack, "", 4, 90)
	if err != nil {
		t.Fatalf("new product: %v", err)
	}
	return p
}

func TestStartCheckoutPricesFromCatalogue(t *testing.T) {
	for _, tt := range []struct {
		name string
		qty  int
		want shared.Money
	}{
		{name: "single unit", qty: 1, want: shared.NewMoney(4700)},
		{name: "quantity scales the unit price", qty: 2, want: shared.NewMoney(9400)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newPurchaseHarness(t)
			ctx := context.Background()
			p := materialProduct(t)
			if err := h.products.Create(ctx, p); err != nil {
				t.Fatal(err)
			}
			userID := shared.ID("user-1")

			url, orderID, err := h.uc.StartCheckout(ctx, userID, p.Slug, tt.qty)
			if err != nil {
				t.Fatalf("StartCheckout: %v", err)
			}
			if url == "" {
				t.Fatal("checkout URL must not be empty")
			}
			order, err := h.orders.GetByID(ctx, orderID)
			if err != nil {
				t.Fatal(err)
			}
			if order.UserID != userID || order.ProductID != p.ID {
				t.Fatalf("order identities: user=%v product=%v", order.UserID, order.ProductID)
			}
			if order.Amount != tt.want {
				t.Fatalf("amount = %d cents, want %d", order.Amount.Cents(), tt.want.Cents())
			}
			if order.Status != ordering.StatusPending {
				t.Fatalf("status = %q, want pending", order.Status)
			}
		})
	}
}

func deliverPaidTwice(t *testing.T, h purchaseHarness, ref string) {
	t.Helper()
	ctx := context.Background()
	const eventID = "evt_1"
	for i := 0; i < 2; i++ {
		if err := h.uc.HandlePaymentWebhook(ctx, eventID, ref, ports.PaymentPaid); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}
}

func TestHandlePaymentWebhookPaidGrantsOneEntitlement(t *testing.T) {
	h := newPurchaseHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	p := materialProduct(t)
	if err := h.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	_, orderID, err := h.uc.StartCheckout(ctx, user.ID, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	deliverPaidTwice(t, h, orderID.String())

	ents, err := h.entitlements.ListByUserID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("got %d entitlements, want exactly 1", len(ents))
	}
	if ents[0].ProductID != p.ID {
		t.Fatalf("entitlement product = %v, want %v", ents[0].ProductID, p.ID)
	}
	order, err := h.orders.GetByID(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != ordering.StatusPaid {
		t.Fatalf("order status = %q, want paid", order.Status)
	}
	if len(h.mailer.PurchaseConfirmedCalls) != 1 {
		t.Fatalf("got %d confirmation mails, want 1", len(h.mailer.PurchaseConfirmedCalls))
	}
	call := h.mailer.PurchaseConfirmedCalls[0]
	if call.To != user.Email || call.Amount != order.Amount {
		t.Fatalf("mail = %+v, want to %s for %d", call, user.Email, order.Amount.Cents())
	}
}

func TestHandlePaymentWebhookPaidGrantsOneCreditBatch(t *testing.T) {
	h := newPurchaseHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	p := packProduct(t)
	if err := h.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	_, orderID, err := h.uc.StartCheckout(ctx, user.ID, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	deliverPaidTwice(t, h, orderID.String())

	ledger, err := h.ledger.Get(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	entries := ledger.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d ledger entries, want exactly 1 credit batch", len(entries))
	}
	if entries[0].Delta != p.Credits {
		t.Fatalf("delta = %d, want %d", entries[0].Delta, p.Credits)
	}
	wantExpiry := h.clock.NowTime.Add(time.Duration(p.ValidityDays) * 24 * time.Hour)
	if !entries[0].ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expiry = %v, want %v", entries[0].ExpiresAt, wantExpiry)
	}
	if len(h.mailer.PurchaseConfirmedCalls) != 1 {
		t.Fatalf("got %d confirmation mails, want 1", len(h.mailer.PurchaseConfirmedCalls))
	}
}

func TestHandlePaymentWebhookFailedTransitionsOrder(t *testing.T) {
	h := newPurchaseHarness(t)
	ctx := context.Background()
	p := materialProduct(t)
	if err := h.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	userID := shared.ID("user-1")
	_, orderID, err := h.uc.StartCheckout(ctx, userID, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.uc.HandlePaymentWebhook(ctx, "evt_2", orderID.String(), ports.PaymentFailed); err != nil {
		t.Fatal(err)
	}
	order, err := h.orders.GetByID(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != ordering.StatusFailed {
		t.Fatalf("status = %q, want failed", order.Status)
	}
	ents, err := h.entitlements.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("failed payment granted %d entitlements", len(ents))
	}
	if len(h.mailer.PurchaseConfirmedCalls) != 0 {
		t.Fatalf("failed payment sent %d mails", len(h.mailer.PurchaseConfirmedCalls))
	}
}

func TestHandlePaymentWebhookUnknownRefErrors(t *testing.T) {
	h := newPurchaseHarness(t)
	err := h.uc.HandlePaymentWebhook(context.Background(), "evt_1", "no-such-order", ports.PaymentPaid)
	if !errors.Is(err, fakes.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListMyOrders(t *testing.T) {
	h := newPurchaseHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	other := shared.ID("user-2")
	p := materialProduct(t)
	if err := h.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, mine, err := h.uc.StartCheckout(ctx, user.ID, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	_, theirs, err := h.uc.StartCheckout(ctx, other, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}

	orders, err := h.uc.ListMyOrders(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) != 1 || orders[0].ID != mine {
		t.Fatalf("got %d orders, want just %v", len(orders), mine)
	}
	if orders[0].ID == theirs {
		t.Fatal("returned another user's order")
	}
}

// Two deliveries of one event can both observe "pending" before either has
// persisted "paid" - that is the whole reason the claim exists. Rewinding the
// order between deliveries reproduces that interleaving deterministically,
// instead of hoping the scheduler opens the window.
func TestHandlePaymentWebhookClaimGuardsAgainstInterleavedRedelivery(t *testing.T) {
	h := newPurchaseHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	p := packProduct(t)
	if err := h.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	_, orderID, err := h.uc.StartCheckout(ctx, user.ID, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := h.uc.HandlePaymentWebhook(ctx, "evt-1", orderID.String(), ports.PaymentPaid); err != nil {
		t.Fatalf("first delivery: %v", err)
	}

	// Rewind to pending: this is what a second in-flight delivery would have
	// read before the first one committed.
	stored, err := h.orders.GetByID(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	rewound := ordering.NewOrder(stored.ID, stored.UserID, stored.ProductID, stored.Amount, stored.CreatedAt)
	if err := h.orders.Save(ctx, rewound); err != nil {
		t.Fatal(err)
	}

	if err := h.uc.HandlePaymentWebhook(ctx, "evt-1", orderID.String(), ports.PaymentPaid); err != nil {
		t.Fatalf("second delivery: %v", err)
	}

	ledger, err := h.ledger.Get(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ledger.Entries()); got != 1 {
		t.Fatalf("got %d ledger entries after a redelivered event, want exactly 1", got)
	}
	if got := len(h.mailer.PurchaseConfirmedCalls); got != 1 {
		t.Fatalf("got %d confirmation mails, want 1", got)
	}
}

// Same event delivered concurrently, under -race: no data race, and still one
// grant. The deterministic test above is what proves the claim logic; this one
// guards the concurrency plumbing.
func TestHandlePaymentWebhookConcurrentRedeliveryGrantsOnce(t *testing.T) {
	h := newPurchaseHarness(t)
	ctx := context.Background()
	user := mustUser(t, "user-1", "ana@test.com")
	p := packProduct(t)
	if err := h.products.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := h.users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	_, orderID, err := h.uc.StartCheckout(ctx, user.ID, p.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}

	const deliveries = 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, deliveries)
	for i := 0; i < deliveries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- h.uc.HandlePaymentWebhook(ctx, "evt-1", orderID.String(), ports.PaymentPaid)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("HandlePaymentWebhook: %v", err)
		}
	}

	ledger, err := h.ledger.Get(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ledger.Entries()); got != 1 {
		t.Fatalf("got %d ledger entries after %d concurrent deliveries, want exactly 1", got, deliveries)
	}
}
