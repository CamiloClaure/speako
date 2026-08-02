package sqlite

import (
	"context"
	"errors"
	"fmt"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/access"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

const teacherID = shared.ID("teacher-1")

// sqliteStore bundles every repo the tests touch, on one real temp-file DB.
type sqliteStore struct {
	tx           *TxManager
	users        *UserRepo
	products     *ProductRepo
	orders       *OrderRepo
	payments     *PaymentEventRepo
	bookings     *BookingRepo
	entitlements *EntitlementRepo
	ledger       *CreditLedgerRepo
}

func newTestStore(t *testing.T) *sqliteStore {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &sqliteStore{
		tx:           NewTxManager(db),
		users:        NewUserRepo(db),
		products:     NewProductRepo(db),
		orders:       NewOrderRepo(db),
		payments:     NewPaymentEventRepo(db),
		bookings:     NewBookingRepo(db, teacherID),
		entitlements: NewEntitlementRepo(db),
		ledger:       NewCreditLedgerRepo(db),
	}
}

func seedUser(t *testing.T, s *sqliteStore, id shared.ID, email string) {
	t.Helper()
	if err := s.users.Create(context.Background(), identity.User{
		ID: id, Email: shared.EmailAddress(email), Name: "Test",
		Role: identity.RoleStudent, Timezone: "America/Sao_Paulo",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func seedProduct(t *testing.T, s *sqliteStore, id shared.ID) {
	t.Helper()
	if err := s.products.Create(context.Background(), catalog.Product{
		ID: id, Slug: string(id), Title: "Test pack", Kind: catalog.KindPack,
		Price: shared.Money(10000), Credits: 5, ValidityDays: 30, Active: true,
	}); err != nil {
		t.Fatalf("create product: %v", err)
	}
}

func TestUserNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.users.GetByID(context.Background(), shared.ID("missing"))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestClaimIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedUser(t, s, shared.ID("u1"), "a@example.com")
	seedProduct(t, s, shared.ID("p1"))
	now := time.Now()
	if err := s.orders.Create(ctx, ordering.NewOrder(
		shared.ID("o1"), shared.ID("u1"), shared.ID("p1"), shared.Money(10000), now,
	)); err != nil {
		t.Fatalf("create order: %v", err)
	}

	first, err := s.payments.Claim(ctx, "evt-1", shared.ID("o1"), now)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	second, err := s.payments.Claim(ctx, "evt-1", shared.ID("o1"), now)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if !first {
		t.Fatal("first claim must win")
	}
	if second {
		t.Fatal("second claim of the same event must lose")
	}
}

func TestConcurrentBookingsSameSlot(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedUser(t, s, shared.ID("student-1"), "s1@example.com")
	seedUser(t, s, shared.ID("student-2"), "s2@example.com")
	seedUser(t, s, teacherID, "teacher@example.com")

	now := time.Now()
	slot := shared.TimeSlot{Start: now.Add(time.Hour).Truncate(time.Minute), End: now.Add(2 * time.Hour).Truncate(time.Minute)}

	const n = 2
	results := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = s.bookings.Create(ctx, scheduling.Booking{
				ID:        shared.ID(fmt.Sprintf("b%d", i)),
				StudentID: shared.ID(fmt.Sprintf("student-%d", i+1)),
				Slot:      slot,
				Status:    scheduling.StatusScheduled,
			})
		}(i)
	}
	wg.Wait()

	successes := 0
	for i, err := range results {
		if err == nil {
			successes++
			continue
		}
		t.Logf("booking %d failed as expected: %v", i, err)
	}
	if successes != 1 {
		t.Fatalf("want exactly 1 success, got %d (results: %v)", successes, results)
	}
}

func TestInTxRollsBackOnError(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	seedProduct(t, s, shared.ID("p1"))

	boom := errors.New("boom")
	err := s.tx.InTx(context.Background(), func(ctx context.Context) error {
		if err := s.users.Create(ctx, identity.User{
			ID: shared.ID("tx-user"), Email: "tx@example.com", Name: "Tx",
			Role: identity.RoleStudent, Timezone: "America/Sao_Paulo",
		}); err != nil {
			return err
		}
		if err := s.entitlements.Add(ctx, access.Entitlement{
			UserID: shared.ID("tx-user"), ProductID: shared.ID("p1"), GrantedAt: now,
		}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}

	if _, err := s.users.GetByID(context.Background(), shared.ID("tx-user")); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("user should be rolled back, got %v", err)
	}
	got, err := s.entitlements.ListByUserID(context.Background(), shared.ID("tx-user"))
	if err != nil {
		t.Fatalf("list entitlements: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("entitlement should be rolled back, got %d rows", len(got))
	}
}

func TestInTxCommits(t *testing.T) {
	s := newTestStore(t)
	err := s.tx.InTx(context.Background(), func(ctx context.Context) error {
		return s.users.Create(ctx, identity.User{
			ID: shared.ID("u-committed"), Email: "committed@example.com", Name: "C",
			Role: identity.RoleStudent, Timezone: "America/Sao_Paulo",
		})
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if _, err := s.users.GetByID(context.Background(), shared.ID("u-committed")); err != nil {
		t.Fatalf("committed user should be visible: %v", err)
	}
}

// A refund, and a pack sold with no validity period, carry no expiry. The
// column is NOT NULL, so writing one as NULL crashed every refund with a 500.
func TestCreditEntryWithoutExpiryRoundTrips(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedUser(t, s, shared.ID("student-1"), "s1@example.com")

	if err := s.ledger.Append(ctx, shared.ID("student-1"), credit.Entry{
		Delta: 1, Reason: "booking_refund", RefID: "b1",
	}); err != nil {
		t.Fatalf("appending a never-expiring credit: %v", err)
	}

	ledger, err := s.ledger.Get(ctx, shared.ID("student-1"))
	if err != nil {
		t.Fatal(err)
	}
	entries := ledger.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !entries[0].ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt = %v, want the zero time (never expires)", entries[0].ExpiresAt)
	}
	if got := ledger.Balance(time.Now().AddDate(10, 0, 0)); got != 1 {
		t.Errorf("balance in ten years = %d, want 1 - a never-expiring credit must not lapse", got)
	}
}

// A slot race is an ordinary outcome, not a server fault: the second student
// must get a domain error the transport can turn into 409, not a raw
// constraint violation that becomes a 500.
func TestCreateBookingOnTakenSlotReturnsDomainError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	seedUser(t, s, shared.ID("student-1"), "s1@example.com")
	seedUser(t, s, shared.ID("student-2"), "s2@example.com")
	seedUser(t, s, teacherID, "teacher@example.com")

	start := time.Now().Add(48 * time.Hour).Truncate(time.Hour)
	slot := shared.TimeSlot{Start: start, End: start.Add(time.Hour)}

	if err := s.bookings.Create(ctx, scheduling.Booking{
		ID: "b1", StudentID: "student-1", Slot: slot, Status: scheduling.StatusScheduled,
	}); err != nil {
		t.Fatalf("first booking: %v", err)
	}
	err := s.bookings.Create(ctx, scheduling.Booking{
		ID: "b2", StudentID: "student-2", Slot: slot, Status: scheduling.StatusScheduled,
	})
	if !errors.Is(err, scheduling.ErrSlotTaken) {
		t.Fatalf("err = %v, want scheduling.ErrSlotTaken", err)
	}
}
