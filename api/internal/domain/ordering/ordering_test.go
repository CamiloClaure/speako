package ordering

import (
	"errors"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

func baseOrder() Order {
	return NewOrder(
		shared.ID("o1"),
		shared.ID("u1"),
		shared.ID("p1"),
		shared.NewMoney(39990),
		time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
	)
}

// transition is one attempt to move an order; wantErr nil means it must succeed.
type transition struct {
	name    string
	from    Status
	apply   func(Order) (Order, error)
	wantErr error
}

func transitions() []transition {
	applyPaid := func(o Order) (Order, error) { return o.MarkPaid(time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC)) }
	return []transition{
		// Legal edges.
		{"pending to paid", StatusPending, applyPaid, nil},
		{"pending to failed", StatusPending, func(o Order) (Order, error) { return o.MarkFailed() }, nil},
		{"pending to expired", StatusPending, func(o Order) (Order, error) { return o.MarkExpired() }, nil},
		{"paid to refunded", StatusPaid, func(o Order) (Order, error) { return o.Refund() }, nil},

		// Every illegal edge.
		{"pending to refunded", StatusPending, func(o Order) (Order, error) { return o.Refund() }, ErrIllegalTransition},
		{"paid to paid", StatusPaid, applyPaid, ErrIllegalTransition},
		{"paid to failed", StatusPaid, func(o Order) (Order, error) { return o.MarkFailed() }, ErrIllegalTransition},
		{"paid to expired", StatusPaid, func(o Order) (Order, error) { return o.MarkExpired() }, ErrIllegalTransition},
		{"failed to paid", StatusFailed, applyPaid, ErrIllegalTransition},
		{"failed to failed", StatusFailed, func(o Order) (Order, error) { return o.MarkFailed() }, ErrIllegalTransition},
		{"failed to expired", StatusFailed, func(o Order) (Order, error) { return o.MarkExpired() }, ErrIllegalTransition},
		{"failed to refunded", StatusFailed, func(o Order) (Order, error) { return o.Refund() }, ErrIllegalTransition},
		{"expired to paid", StatusExpired, applyPaid, ErrIllegalTransition},
		{"expired to failed", StatusExpired, func(o Order) (Order, error) { return o.MarkFailed() }, ErrIllegalTransition},
		{"expired to expired", StatusExpired, func(o Order) (Order, error) { return o.MarkExpired() }, ErrIllegalTransition},
		{"expired to refunded", StatusExpired, func(o Order) (Order, error) { return o.Refund() }, ErrIllegalTransition},
		{"refunded to paid", StatusRefunded, applyPaid, ErrIllegalTransition},
		{"refunded to failed", StatusRefunded, func(o Order) (Order, error) { return o.MarkFailed() }, ErrIllegalTransition},
		{"refunded to expired", StatusRefunded, func(o Order) (Order, error) { return o.MarkExpired() }, ErrIllegalTransition},
		{"refunded to refunded", StatusRefunded, func(o Order) (Order, error) { return o.Refund() }, ErrIllegalTransition},
	}
}

func TestOrderTransitions(t *testing.T) {
	for _, tt := range transitions() {
		t.Run(tt.name, func(t *testing.T) {
			o := baseOrder()
			o.Status = tt.from
			got, err := tt.apply(o)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if got.Status == tt.from {
					t.Fatalf("status stayed %q, want it to have moved", tt.from)
				}
				if got != o && got.Status == tt.from {
					t.Fatalf("transition produced no new order")
				}
			} else if got.Status != tt.from {
				t.Fatalf("failed transition mutated status to %q, want unchanged %q", got.Status, tt.from)
			}
		})
	}
}

func TestMarkPaidStampsPaidAt(t *testing.T) {
	paidAt := time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC)
	o, err := baseOrder().MarkPaid(paidAt)
	if err != nil {
		t.Fatal(err)
	}
	if !o.PaidAt.Equal(paidAt) {
		t.Fatalf("PaidAt = %v, want %v", o.PaidAt, paidAt)
	}
	if o.Status != StatusPaid {
		t.Fatalf("Status = %q, want paid", o.Status)
	}
}

func TestWithdrawalWindow(t *testing.T) {
	paidAt := time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC)
	paid := baseOrder()
	paid.Status = StatusPaid
	paid.PaidAt = paidAt

	windowEnd := paidAt.Add(WithdrawalWindow) // exactly 7 days later

	tests := []struct {
		name string
		o    Order
		now  time.Time
		want bool
	}{
		{"unpaid order never within", baseOrder(), paidAt, false},
		{"before paid_at is outside", paid, paidAt.Add(-time.Second), false},
		{"at paid_at is inside", paid, paidAt, true},
		{"mid window is inside", paid, paidAt.Add(3 * 24 * time.Hour), true},
		{"one second before end is inside", paid, windowEnd.Add(-time.Second), true},
		{"at exactly seven days is outside", paid, windowEnd, false},
		{"after window is outside", paid, windowEnd.Add(time.Second), false},
		{"refunded order is outside", paidWithStatus(StatusRefunded, paidAt), paidAt.Add(time.Hour), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.IsWithinWithdrawalWindow(tt.now); got != tt.want {
				t.Fatalf("IsWithinWithdrawalWindow(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func paidWithStatus(s Status, paidAt time.Time) Order {
	o := baseOrder()
	o.Status = s
	o.PaidAt = paidAt
	return o
}
