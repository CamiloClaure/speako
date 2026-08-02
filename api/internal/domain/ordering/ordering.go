package ordering

import (
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrIllegalTransition is returned when an order tries to move along an edge
// that does not exist in the state machine.
var ErrIllegalTransition = shared.NewDomainError("order.transition", "illegal order transition")

// Status is a node in the order state machine:
//
//	pending → paid | failed | expired
//	paid    → refunded
type Status string

const (
	StatusPending  Status = "pending"
	StatusPaid     Status = "paid"
	StatusFailed   Status = "failed"
	StatusExpired  Status = "expired"
	StatusRefunded Status = "refunded"
)

// WithdrawalWindow is the CDC art. 49 right-of-withdrawal period.
const WithdrawalWindow = 7 * 24 * time.Hour

// Order is the purchase aggregate. PaidAt records when the payment cleared;
// the withdrawal window counts from it.
//
// ProviderRef is the gateway's own identifier for this checkout. The domain
// never interprets it - it is carried so that reconciliation can ask the
// gateway about an order that never produced a webhook.
type Order struct {
	ID          shared.ID
	UserID      shared.ID
	ProductID   shared.ID
	Amount      shared.Money
	Status      Status
	CreatedAt   time.Time
	PaidAt      time.Time
	ProviderRef string
}

// WithProviderRef returns the order tagged with the gateway's reference.
func (o Order) WithProviderRef(ref string) Order {
	o.ProviderRef = ref
	return o
}

// NewOrder starts an order in the pending state.
func NewOrder(id, userID, productID shared.ID, amount shared.Money, createdAt time.Time) Order {
	return Order{
		ID:        id,
		UserID:    userID,
		ProductID: productID,
		Amount:    amount,
		Status:    StatusPending,
		CreatedAt: createdAt,
	}
}

// MarkPaid moves pending → paid and stamps the payment time.
func (o Order) MarkPaid(now time.Time) (Order, error) {
	if o.Status != StatusPending {
		return o, ErrIllegalTransition
	}
	o.Status = StatusPaid
	o.PaidAt = now
	return o, nil
}

// MarkFailed moves pending → failed.
func (o Order) MarkFailed() (Order, error) {
	if o.Status != StatusPending {
		return o, ErrIllegalTransition
	}
	o.Status = StatusFailed
	return o, nil
}

// MarkExpired moves pending → expired.
func (o Order) MarkExpired() (Order, error) {
	if o.Status != StatusPending {
		return o, ErrIllegalTransition
	}
	o.Status = StatusExpired
	return o, nil
}

// Refund moves paid → refunded.
func (o Order) Refund() (Order, error) {
	if o.Status != StatusPaid {
		return o, ErrIllegalTransition
	}
	o.Status = StatusRefunded
	return o, nil
}

// IsWithinWithdrawalWindow reports whether now falls inside the seven-day
// right-of-withdrawal window counted from PaidAt. The window is open from
// PaidAt (inclusive) until PaidAt + 7 days (exclusive).
func (o Order) IsWithinWithdrawalWindow(now time.Time) bool {
	if o.Status != StatusPaid || o.PaidAt.IsZero() {
		return false
	}
	return !now.Before(o.PaidAt) && now.Before(o.PaidAt.Add(WithdrawalWindow))
}
