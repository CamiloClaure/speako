package ports

import (
	"context"
	"errors"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/access"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/leads"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// UserRepo stores users.
type UserRepo interface {
	Create(ctx context.Context, u identity.User) error
	GetByID(ctx context.Context, id shared.ID) (identity.User, error)
	GetByEmail(ctx context.Context, email shared.EmailAddress) (identity.User, error)
}

// ErrNotFound is what every repository returns when a record is absent, and
// only then. It has to be distinguishable from a real storage failure: code
// that treats "the database is down" as "no such account" silently swallows
// outages and, worse, goes on to create duplicate records.
var ErrNotFound = errors.New("not found")

// SessionRepo stores live sessions. Session.Token holds the hash of the cookie
// value, never the raw token, so a database leak cannot be replayed as a login.
type SessionRepo interface {
	Create(ctx context.Context, s identity.Session) error
	GetByTokenHash(ctx context.Context, tokenHash shared.ID) (identity.Session, error)
	Delete(ctx context.Context, tokenHash shared.ID) error
	DeleteExpired(ctx context.Context, before time.Time) error
}

// LoginToken is a single-use magic-link token, stored hashed and consumed on
// first use so a forwarded or logged link cannot be replayed.
type LoginToken struct {
	TokenHash string
	Email     shared.EmailAddress
	ExpiresAt time.Time
	UsedAt    time.Time
}

// LoginTokenRepo stores magic-link tokens. Consume must atomically mark the
// token used and report whether this caller was the one that claimed it.
type LoginTokenRepo interface {
	Create(ctx context.Context, t LoginToken) error
	Consume(ctx context.Context, tokenHash string, now time.Time) (LoginToken, error)
	DeleteExpired(ctx context.Context, before time.Time) error
}

// ProductRepo stores the catalogue of materials and packs.
type ProductRepo interface {
	Create(ctx context.Context, p catalog.Product) error
	Update(ctx context.Context, p catalog.Product) error
	GetByID(ctx context.Context, id shared.ID) (catalog.Product, error)
	GetBySlug(ctx context.Context, slug string) (catalog.Product, error)
	List(ctx context.Context) ([]catalog.Product, error)
}

// OrderRepo stores purchase orders and their lifecycle state.
type OrderRepo interface {
	Create(ctx context.Context, o ordering.Order) error
	Save(ctx context.Context, o ordering.Order) error
	GetByID(ctx context.Context, id shared.ID) (ordering.Order, error)
	ListByUserID(ctx context.Context, userID shared.ID) ([]ordering.Order, error)
	ListPendingBefore(ctx context.Context, before time.Time) ([]ordering.Order, error)
	ListAll(ctx context.Context) ([]ordering.Order, error)
}

// PaymentEventRepo records which gateway events have already been applied.
//
// This is what makes webhook handling safe, and it has to be a storage-level
// claim rather than a status check: gateways retry aggressively, and two
// deliveries of the same event can be in flight at once. Checking the order
// status first would let both readers see "pending" and both grant credits.
type PaymentEventRepo interface {
	// Claim records eventID as being handled and reports whether this caller
	// won the race. A second call for the same eventID returns false. It must
	// be called inside the same transaction as the work it guards.
	Claim(ctx context.Context, eventID string, orderID shared.ID, now time.Time) (claimed bool, err error)
}

// EntitlementRepo stores the fact that a user may open a material. Granting
// idempotency lives in the domain (access.GrantEntitlement), not here.
type EntitlementRepo interface {
	Add(ctx context.Context, e access.Entitlement) error
	ListByUserID(ctx context.Context, userID shared.ID) ([]access.Entitlement, error)
}

// CreditLedgerRepo stores one append-only credit ledger per user.
type CreditLedgerRepo interface {
	Append(ctx context.Context, userID shared.ID, e credit.Entry) error
	Get(ctx context.Context, userID shared.ID) (credit.Ledger, error)
}

// BookingRepo stores lesson bookings.
type BookingRepo interface {
	Create(ctx context.Context, b scheduling.Booking) error
	Save(ctx context.Context, b scheduling.Booking) error
	GetByID(ctx context.Context, id shared.ID) (scheduling.Booking, error)
	ListByStudentID(ctx context.Context, studentID shared.ID) ([]scheduling.Booking, error)
	ListBetween(ctx context.Context, from, to time.Time) ([]scheduling.Booking, error)
}

// AvailabilityRepo stores the teacher's weekly availability rules and their
// per-date exceptions. The platform runs a single teacher, so no user scoping.
type AvailabilityRepo interface {
	SaveRules(ctx context.Context, rules []scheduling.AvailabilityRule) error
	ListRules(ctx context.Context) ([]scheduling.AvailabilityRule, error)
	SaveException(ctx context.Context, e scheduling.Exception) error
	RemoveException(ctx context.Context, date time.Time) error
	ListExceptions(ctx context.Context) ([]scheduling.Exception, error)
}

// LeadRepo stores captured prospects awaiting double opt-in.
type LeadRepo interface {
	Create(ctx context.Context, l leads.Lead) error
	Save(ctx context.Context, l leads.Lead) error
	GetByEmail(ctx context.Context, email shared.EmailAddress) (leads.Lead, error)
	GetByConfirmTokenHash(ctx context.Context, hash string) (leads.Lead, error)
	List(ctx context.Context) ([]leads.Lead, error)
}

// PaymentStatus is a payment's state as reported by the gateway. It is gateway
// vocabulary, distinct from ordering.Status; use cases translate it into order
// transitions.
type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentPaid     PaymentStatus = "paid"
	PaymentFailed   PaymentStatus = "failed"
	PaymentExpired  PaymentStatus = "expired"
	PaymentRefunded PaymentStatus = "refunded"
)

// PaymentGateway starts checkouts and reports on their state. The returned ref
// is what the webhook echoes back; the integration sets it to order.ID so the
// order stays recoverable without extra state.
type PaymentGateway interface {
	CreateCheckout(ctx context.Context, order ordering.Order) (url string, ref string, err error)
	FetchPayment(ctx context.Context, ref string) (PaymentStatus, error)
}

// FileStorage hands out temporary download URLs for stored objects.
type FileStorage interface {
	SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// Mailer sends transactional email. One method per message type, so use cases
// cannot slip unstructured HTML past the caller.
type Mailer interface {
	MagicLink(ctx context.Context, to shared.EmailAddress, url string) error
	PurchaseConfirmed(ctx context.Context, to shared.EmailAddress, productTitle string, amount shared.Money) error
	BookingConfirmed(ctx context.Context, to shared.EmailAddress, slot shared.TimeSlot) error
	BookingCancelled(ctx context.Context, to shared.EmailAddress, slot shared.TimeSlot) error
	LessonReminder(ctx context.Context, to shared.EmailAddress, slot shared.TimeSlot) error
	LeadOptIn(ctx context.Context, to shared.EmailAddress, confirmURL string) error
}

// Clock tells the current time; injected so use cases stay deterministic.
type Clock interface {
	Now() time.Time
}

// IDGen produces new public identifiers (UUIDv7-shaped in production).
type IDGen interface {
	NewID() shared.ID
}

// TokenGen produces a raw secret token and its hash. Only the hash is stored;
// the raw value travels in the magic link and is never persisted.
//
// Hash exists so there is exactly one hashing implementation. When a caller
// re-derives the hash itself to look a token up, the two implementations can
// drift and every login silently stops matching.
type TokenGen interface {
	NewToken() (raw string, hash string, err error)
	Hash(raw string) string
}

// TxManager runs a function inside a database transaction; the returned error
// rolls back whatever the function wrote.
type TxManager interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}
