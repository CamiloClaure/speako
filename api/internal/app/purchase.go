package app

import (
	"context"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/access"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// Purchase runs the checkout and order lifecycle. The order amount always
// comes from the catalogue, never from a caller; a webhook only moves an
// order, it never reports a price.
type Purchase struct {
	products     ports.ProductRepo
	orders       ports.OrderRepo
	users        ports.UserRepo
	entitlements ports.EntitlementRepo
	ledger       ports.CreditLedgerRepo
	events       ports.PaymentEventRepo
	payments     ports.PaymentGateway
	mailer       ports.Mailer
	tx           ports.TxManager
	clock        ports.Clock
	idgen        ports.IDGen
}

// NewPurchase wires the checkout use cases to their ports.
func NewPurchase(
	products ports.ProductRepo,
	orders ports.OrderRepo,
	users ports.UserRepo,
	entitlements ports.EntitlementRepo,
	ledger ports.CreditLedgerRepo,
	events ports.PaymentEventRepo,
	payments ports.PaymentGateway,
	mailer ports.Mailer,
	tx ports.TxManager,
	clock ports.Clock,
	idgen ports.IDGen,
) *Purchase {
	return &Purchase{
		products:     products,
		orders:       orders,
		users:        users,
		entitlements: entitlements,
		ledger:       ledger,
		events:       events,
		payments:     payments,
		mailer:       mailer,
		tx:           tx,
		clock:        clock,
		idgen:        idgen,
	}
}

// StartCheckout builds a pending order priced from the catalogue (quantity
// scales the unit price), opens a gateway checkout and returns its URL and
// the order id. The gateway reference is the order id by port contract, so
// the webhook can recover the order without extra state.
func (uc *Purchase) StartCheckout(ctx context.Context, userID shared.ID, productSlug string, quantity int) (string, shared.ID, error) {
	product, err := uc.products.GetBySlug(ctx, productSlug)
	if err != nil {
		return "", shared.ID(""), err
	}
	amount, err := product.Price.Mul(quantity)
	if err != nil {
		return "", shared.ID(""), err
	}
	order := ordering.NewOrder(uc.idgen.NewID(), userID, product.ID, amount, uc.clock.Now())
	if err := uc.orders.Create(ctx, order); err != nil {
		return "", shared.ID(""), err
	}
	url, ref, err := uc.payments.CreateCheckout(ctx, order)
	if err != nil {
		return "", shared.ID(""), err
	}
	// Persist the gateway's reference: without it, an order whose webhook never
	// arrives cannot be reconciled and the student stays unpaid-but-charged.
	if err := uc.orders.Save(ctx, order.WithProviderRef(ref)); err != nil {
		return "", shared.ID(""), err
	}
	return url, order.ID, nil
}

// HandlePaymentWebhook applies a gateway-reported status to the order that
// owns ref. Idempotency is enforced by claiming eventID inside the same
// transaction as the grant, so redelivering an event - even concurrently -
// grants nothing twice. The order and its amount are re-read from storage;
// the caller never supplies an amount.
func (uc *Purchase) HandlePaymentWebhook(ctx context.Context, eventID, ref string, status ports.PaymentStatus) error {
	order, err := uc.orders.GetByID(ctx, shared.ID(ref))
	if err != nil {
		return err
	}
	if order.Status != ordering.StatusPending {
		return nil // already processed; a duplicate delivery must change nothing
	}
	now := uc.clock.Now()
	switch status {
	case ports.PaymentPaid:
		paid, err := order.MarkPaid(now)
		if err != nil {
			return err
		}
		product, err := uc.products.GetByID(ctx, order.ProductID)
		if err != nil {
			return err
		}
		granted := false
		if err := uc.tx.InTx(ctx, func(ctx context.Context) error {
			// Claim the event inside the transaction. The status check above is
			// only a cheap filter - two concurrent deliveries can both read
			// "pending", and without this claim both would grant credits.
			claimed, err := uc.events.Claim(ctx, eventID, order.ID, now)
			if err != nil {
				return err
			}
			if !claimed {
				return nil
			}
			granted = true
			if err := uc.orders.Save(ctx, paid); err != nil {
				return err
			}
			switch {
			case product.Kind.IsMaterial():
				existing, err := uc.entitlements.ListByUserID(ctx, order.UserID)
				if err != nil {
					return err
				}
				withNew := access.GrantEntitlement(existing, order.UserID, product.ID, now)
				if len(withNew) > len(existing) {
					if err := uc.entitlements.Add(ctx, withNew[len(withNew)-1]); err != nil {
						return err
					}
				}
			case product.Kind.IsPack():
				entry := credit.Entry{
					Delta:     product.Credits,
					Reason:    "purchase",
					RefID:     order.ID.String(),
					ExpiresAt: packExpiry(now, product.ValidityDays),
				}
				if err := uc.ledger.Append(ctx, order.UserID, entry); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if !granted {
			return nil // another delivery of this event already did the work
		}
		user, err := uc.users.GetByID(ctx, order.UserID)
		if err != nil {
			return err
		}
		return uc.mailer.PurchaseConfirmed(ctx, user.Email, product.Title, order.Amount)
	case ports.PaymentFailed:
		failed, err := order.MarkFailed()
		if err != nil {
			return err
		}
		return uc.orders.Save(ctx, failed)
	case ports.PaymentExpired:
		expired, err := order.MarkExpired()
		if err != nil {
			return err
		}
		return uc.orders.Save(ctx, expired)
	default:
		return nil // pending or unknown statuses change nothing
	}
}

// ListMyOrders returns every order the user has placed, oldest first.
func (uc *Purchase) ListMyOrders(ctx context.Context, userID shared.ID) ([]ordering.Order, error) {
	return uc.orders.ListByUserID(ctx, userID)
}

// packExpiry converts a pack's validity into a ledger expiry. Zero means the
// credits never expire.
func packExpiry(purchasedAt time.Time, validityDays int) time.Time {
	if validityDays == 0 {
		return time.Time{}
	}
	return purchasedAt.Add(time.Duration(validityDays) * 24 * time.Hour)
}
