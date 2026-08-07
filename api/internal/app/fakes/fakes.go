package fakes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/access"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/leads"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
)

// ErrNotFound is returned by a fake Get when the key is absent. It wraps the
// port sentinel so use cases can tell "absent" from "storage broke".
var ErrNotFound = fmt.Errorf("fakes: %w", ports.ErrNotFound)

// UserRepo is an in-memory UserRepo. Set Err to force every method to fail.
type UserRepo struct {
	mu           sync.Mutex
	byID         map[shared.ID]identity.User
	byEmail      map[shared.EmailAddress]shared.ID
	passwordHash map[shared.ID]string
	Err          error
}

func (r *UserRepo) init() {
	if r.byID == nil {
		r.byID = map[shared.ID]identity.User{}
		r.byEmail = map[shared.EmailAddress]shared.ID{}
		r.passwordHash = map[shared.ID]string{}
	}
}

func (r *UserRepo) Create(ctx context.Context, u identity.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.byID[u.ID] = u
	r.byEmail[u.Email] = u.ID
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id shared.ID) (identity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return identity.User{}, r.Err
	}
	u, ok := r.byID[id]
	if !ok {
		return identity.User{}, ErrNotFound
	}
	return u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email shared.EmailAddress) (identity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return identity.User{}, r.Err
	}
	id, ok := r.byEmail[email]
	if !ok {
		return identity.User{}, ErrNotFound
	}
	return r.byID[id], nil
}

// GetPasswordHash returns an empty string for a user with no password set,
// matching the SQLite adapter's NOT NULL DEFAULT empty-string column.
func (r *UserRepo) GetPasswordHash(ctx context.Context, userID shared.ID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return "", r.Err
	}
	r.init()
	if _, ok := r.byID[userID]; !ok {
		return "", ErrNotFound
	}
	return r.passwordHash[userID], nil
}

func (r *UserRepo) SetPasswordHash(ctx context.Context, userID shared.ID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.passwordHash[userID] = hash
	return nil
}

// ProductRepo is an in-memory ProductRepo. Set Err to force every method to
// fail. List returns products in insertion order.
type ProductRepo struct {
	mu     sync.Mutex
	byID   map[shared.ID]catalog.Product
	bySlug map[string]shared.ID
	order  []shared.ID
	Err    error
}

func (r *ProductRepo) init() {
	if r.byID == nil {
		r.byID = map[shared.ID]catalog.Product{}
		r.bySlug = map[string]shared.ID{}
	}
}

func (r *ProductRepo) put(p catalog.Product) {
	if _, ok := r.byID[p.ID]; !ok {
		r.order = append(r.order, p.ID)
	}
	r.byID[p.ID] = p
	r.bySlug[p.Slug] = p.ID
}

func (r *ProductRepo) Create(ctx context.Context, p catalog.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(p)
	return nil
}

func (r *ProductRepo) Update(ctx context.Context, p catalog.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(p)
	return nil
}

func (r *ProductRepo) GetByID(ctx context.Context, id shared.ID) (catalog.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return catalog.Product{}, r.Err
	}
	p, ok := r.byID[id]
	if !ok {
		return catalog.Product{}, ErrNotFound
	}
	return p, nil
}

func (r *ProductRepo) GetBySlug(ctx context.Context, slug string) (catalog.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return catalog.Product{}, r.Err
	}
	id, ok := r.bySlug[slug]
	if !ok {
		return catalog.Product{}, ErrNotFound
	}
	return r.byID[id], nil
}

func (r *ProductRepo) List(ctx context.Context) ([]catalog.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	out := make([]catalog.Product, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out, nil
}

// OrderRepo is an in-memory OrderRepo. Set Err to force every method to fail.
// List methods return rows in insertion order.
type OrderRepo struct {
	mu    sync.Mutex
	byID  map[shared.ID]ordering.Order
	order []shared.ID
	Err   error
}

func (r *OrderRepo) init() {
	if r.byID == nil {
		r.byID = map[shared.ID]ordering.Order{}
	}
}

func (r *OrderRepo) put(o ordering.Order) {
	if _, ok := r.byID[o.ID]; !ok {
		r.order = append(r.order, o.ID)
	}
	r.byID[o.ID] = o
}

func (r *OrderRepo) Create(ctx context.Context, o ordering.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(o)
	return nil
}

func (r *OrderRepo) Save(ctx context.Context, o ordering.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(o)
	return nil
}

func (r *OrderRepo) GetByID(ctx context.Context, id shared.ID) (ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return ordering.Order{}, r.Err
	}
	o, ok := r.byID[id]
	if !ok {
		return ordering.Order{}, ErrNotFound
	}
	return o, nil
}

func (r *OrderRepo) ListByUserID(ctx context.Context, userID shared.ID) ([]ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	var out []ordering.Order
	for _, id := range r.order {
		if o, ok := r.byID[id]; ok && o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *OrderRepo) ListPendingBefore(ctx context.Context, before time.Time) ([]ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	var out []ordering.Order
	for _, id := range r.order {
		if o, ok := r.byID[id]; ok && o.Status == ordering.StatusPending && o.CreatedAt.Before(before) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (r *OrderRepo) ListAll(ctx context.Context) ([]ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	out := make([]ordering.Order, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out, nil
}

// EntitlementRepo is an in-memory EntitlementRepo. Set Err to force every
// method to fail.
type EntitlementRepo struct {
	mu     sync.Mutex
	byUser map[shared.ID][]access.Entitlement
	Err    error
}

func (r *EntitlementRepo) init() {
	if r.byUser == nil {
		r.byUser = map[shared.ID][]access.Entitlement{}
	}
}

func (r *EntitlementRepo) Add(ctx context.Context, e access.Entitlement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.byUser[e.UserID] = append(r.byUser[e.UserID], e)
	return nil
}

func (r *EntitlementRepo) ListByUserID(ctx context.Context, userID shared.ID) ([]access.Entitlement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	return append([]access.Entitlement(nil), r.byUser[userID]...), nil
}

// CreditLedgerRepo is an in-memory CreditLedgerRepo. Set Err to force every
// method to fail.
type CreditLedgerRepo struct {
	mu     sync.Mutex
	byUser map[shared.ID]credit.Ledger
	Err    error
}

func (r *CreditLedgerRepo) init() {
	if r.byUser == nil {
		r.byUser = map[shared.ID]credit.Ledger{}
	}
}

func (r *CreditLedgerRepo) Append(ctx context.Context, userID shared.ID, e credit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.byUser[userID] = r.byUser[userID].Append(e)
	return nil
}

func (r *CreditLedgerRepo) Get(ctx context.Context, userID shared.ID) (credit.Ledger, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return credit.Ledger{}, r.Err
	}
	return r.byUser[userID], nil
}

// BookingRepo is an in-memory BookingRepo. Set Err to force every method to
// fail. List methods return rows in insertion order.
type BookingRepo struct {
	mu    sync.Mutex
	byID  map[shared.ID]scheduling.Booking
	order []shared.ID
	Err   error
}

func (r *BookingRepo) init() {
	if r.byID == nil {
		r.byID = map[shared.ID]scheduling.Booking{}
	}
}

func (r *BookingRepo) put(b scheduling.Booking) {
	if _, ok := r.byID[b.ID]; !ok {
		r.order = append(r.order, b.ID)
	}
	r.byID[b.ID] = b
}

func (r *BookingRepo) Create(ctx context.Context, b scheduling.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(b)
	return nil
}

func (r *BookingRepo) Save(ctx context.Context, b scheduling.Booking) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(b)
	return nil
}

func (r *BookingRepo) GetByID(ctx context.Context, id shared.ID) (scheduling.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return scheduling.Booking{}, r.Err
	}
	b, ok := r.byID[id]
	if !ok {
		return scheduling.Booking{}, ErrNotFound
	}
	return b, nil
}

func (r *BookingRepo) ListByStudentID(ctx context.Context, studentID shared.ID) ([]scheduling.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	var out []scheduling.Booking
	for _, id := range r.order {
		if b, ok := r.byID[id]; ok && b.StudentID == studentID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *BookingRepo) ListBetween(ctx context.Context, from, to time.Time) ([]scheduling.Booking, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	var out []scheduling.Booking
	for _, id := range r.order {
		b := r.byID[id]
		if b.Slot.Start.Before(to) && b.Slot.End.After(from) {
			out = append(out, b)
		}
	}
	return out, nil
}

// AvailabilityRepo is an in-memory AvailabilityRepo. Set Err to force every
// method to fail.
type AvailabilityRepo struct {
	mu         sync.Mutex
	rules      []scheduling.AvailabilityRule
	exceptions []scheduling.Exception
	Err        error
}

func (r *AvailabilityRepo) SaveRules(ctx context.Context, rules []scheduling.AvailabilityRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.rules = append([]scheduling.AvailabilityRule(nil), rules...)
	return nil
}

func (r *AvailabilityRepo) ListRules(ctx context.Context) ([]scheduling.AvailabilityRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	return append([]scheduling.AvailabilityRule(nil), r.rules...), nil
}

func (r *AvailabilityRepo) SaveException(ctx context.Context, e scheduling.Exception) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	for i, x := range r.exceptions {
		if sameDate(x.Date, e.Date) {
			r.exceptions[i] = e
			return nil
		}
	}
	r.exceptions = append(r.exceptions, e)
	return nil
}

func (r *AvailabilityRepo) RemoveException(ctx context.Context, date time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	kept := r.exceptions[:0]
	for _, x := range r.exceptions {
		if !sameDate(x.Date, date) {
			kept = append(kept, x)
		}
	}
	r.exceptions = kept
	return nil
}

func (r *AvailabilityRepo) ListExceptions(ctx context.Context) ([]scheduling.Exception, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	return append([]scheduling.Exception(nil), r.exceptions...), nil
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// LeadRepo is an in-memory LeadRepo. Set Err to force every method to fail.
// List returns leads in insertion order.
type LeadRepo struct {
	mu      sync.Mutex
	byEmail map[shared.EmailAddress]leads.Lead
	order   []shared.EmailAddress
	Err     error
}

func (r *LeadRepo) init() {
	if r.byEmail == nil {
		r.byEmail = map[shared.EmailAddress]leads.Lead{}
	}
}

func (r *LeadRepo) put(l leads.Lead) {
	if _, ok := r.byEmail[l.Email]; !ok {
		r.order = append(r.order, l.Email)
	}
	r.byEmail[l.Email] = l
}

func (r *LeadRepo) Create(ctx context.Context, l leads.Lead) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(l)
	return nil
}

func (r *LeadRepo) Save(ctx context.Context, l leads.Lead) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return r.Err
	}
	r.init()
	r.put(l)
	return nil
}

func (r *LeadRepo) GetByEmail(ctx context.Context, email shared.EmailAddress) (leads.Lead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return leads.Lead{}, r.Err
	}
	l, ok := r.byEmail[email]
	if !ok {
		return leads.Lead{}, ErrNotFound
	}
	return l, nil
}

func (r *LeadRepo) GetByConfirmTokenHash(ctx context.Context, hash string) (leads.Lead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return leads.Lead{}, r.Err
	}
	for _, l := range r.byEmail {
		if l.ConfirmTokenHash != "" && l.ConfirmTokenHash == hash {
			return l, nil
		}
	}
	return leads.Lead{}, ErrNotFound
}

func (r *LeadRepo) List(ctx context.Context) ([]leads.Lead, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Err != nil {
		return nil, r.Err
	}
	out := make([]leads.Lead, 0, len(r.order))
	for _, email := range r.order {
		out = append(out, r.byEmail[email])
	}
	return out, nil
}

// PaymentGateway fakes a checkout provider. CreateCheckout returns
// CheckoutURL (a default per-ref URL when empty) and uses order.ID as the
// reference, mirroring the port contract. Pre-seed payment states with
// SetPayment; FetchPayment returns ErrNotFound for unknown refs. Set Err to
// fail CreateCheckout and FetchErr to fail FetchPayment.
type PaymentGateway struct {
	mu          sync.Mutex
	CheckoutURL string
	byRef       map[string]ordering.Order
	payments    map[string]ports.PaymentStatus
	Err         error
	FetchErr    error
}

func (g *PaymentGateway) init() {
	if g.byRef == nil {
		g.byRef = map[string]ordering.Order{}
		g.payments = map[string]ports.PaymentStatus{}
	}
}

func (g *PaymentGateway) CreateCheckout(ctx context.Context, order ordering.Order) (string, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Err != nil {
		return "", "", g.Err
	}
	g.init()
	ref := order.ID.String()
	g.byRef[ref] = order
	if g.payments[ref] == "" {
		g.payments[ref] = ports.PaymentPending
	}
	url := g.CheckoutURL
	if url == "" {
		url = "https://checkout.test/" + ref
	}
	return url, ref, nil
}

func (g *PaymentGateway) FetchPayment(ctx context.Context, ref string) (ports.PaymentStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.FetchErr != nil {
		return "", g.FetchErr
	}
	status, ok := g.payments[ref]
	if !ok {
		return "", ErrNotFound
	}
	return status, nil
}

// SetPayment forces the reported state of the payment referenced by ref.
func (g *PaymentGateway) SetPayment(ref string, status ports.PaymentStatus) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.init()
	g.payments[ref] = status
}

// FileStorage fakes a blob store. SignedURL returns the mapped URL for a key
// or a deterministic default. Set Err to force failures.
type FileStorage struct {
	mu   sync.Mutex
	URLs map[string]string
	Err  error
}

func (s *FileStorage) SignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return "", s.Err
	}
	if u, ok := s.URLs[key]; ok {
		return u, nil
	}
	return "https://cdn.test/" + key, nil
}

// MagicLinkCall records one MagicLink mail.
type MagicLinkCall struct {
	To  shared.EmailAddress
	URL string
}

// PurchaseConfirmedCall records one PurchaseConfirmed mail.
type PurchaseConfirmedCall struct {
	To           shared.EmailAddress
	ProductTitle string
	Amount       shared.Money
}

// BookingEmailCall records a mail about one booking slot.
type BookingEmailCall struct {
	To   shared.EmailAddress
	Slot shared.TimeSlot
}

// LeadOptInCall records one double opt-in mail.
type LeadOptInCall struct {
	To         shared.EmailAddress
	ConfirmURL string
}

// Mailer fakes the transactional email service. It appends one record per
// call, which tests assert against. Set Err to force failures.
type Mailer struct {
	mu                     sync.Mutex
	Err                    error
	MagicLinkCalls         []MagicLinkCall
	PurchaseConfirmedCalls []PurchaseConfirmedCall
	BookingConfirmedCalls  []BookingEmailCall
	BookingCancelledCalls  []BookingEmailCall
	LessonReminderCalls    []BookingEmailCall
	LeadOptInCalls         []LeadOptInCall
}

func (m *Mailer) MagicLink(ctx context.Context, to shared.EmailAddress, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.MagicLinkCalls = append(m.MagicLinkCalls, MagicLinkCall{To: to, URL: url})
	return nil
}

func (m *Mailer) PurchaseConfirmed(ctx context.Context, to shared.EmailAddress, productTitle string, amount shared.Money) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.PurchaseConfirmedCalls = append(m.PurchaseConfirmedCalls, PurchaseConfirmedCall{
		To: to, ProductTitle: productTitle, Amount: amount,
	})
	return nil
}

func (m *Mailer) BookingConfirmed(ctx context.Context, to shared.EmailAddress, slot shared.TimeSlot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.BookingConfirmedCalls = append(m.BookingConfirmedCalls, BookingEmailCall{To: to, Slot: slot})
	return nil
}

func (m *Mailer) BookingCancelled(ctx context.Context, to shared.EmailAddress, slot shared.TimeSlot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.BookingCancelledCalls = append(m.BookingCancelledCalls, BookingEmailCall{To: to, Slot: slot})
	return nil
}

func (m *Mailer) LessonReminder(ctx context.Context, to shared.EmailAddress, slot shared.TimeSlot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.LessonReminderCalls = append(m.LessonReminderCalls, BookingEmailCall{To: to, Slot: slot})
	return nil
}

func (m *Mailer) LeadOptIn(ctx context.Context, to shared.EmailAddress, confirmURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.LeadOptInCalls = append(m.LeadOptInCalls, LeadOptInCall{To: to, ConfirmURL: confirmURL})
	return nil
}

// Clock is a fixed Clock; set NowTime to the instant tests want.
type Clock struct {
	NowTime time.Time
}

func (c *Clock) Now() time.Time { return c.NowTime }

// IDGen hands out deterministic UUIDv7-shaped identifiers.
type IDGen struct {
	mu  sync.Mutex
	seq int
}

func (g *IDGen) NewID() shared.ID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return shared.ID(fmt.Sprintf("00000000-0000-4000-8000-%012d", g.seq))
}

// TokenGen hands out deterministic raw tokens and their SHA-256 hashes. Set
// Err to force failures.
type TokenGen struct {
	mu  sync.Mutex
	seq int
	Err error
}

// Hash is the single hashing implementation the fake exposes; auth uses it
// rather than re-deriving the hash, so the two can never drift.
func (g *TokenGen) Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (g *TokenGen) NewToken() (string, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Err != nil {
		return "", "", g.Err
	}
	g.seq++
	raw := fmt.Sprintf("tok_%d", g.seq)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

// TxManager runs the function immediately. Set Err to fail without running
// the transaction body.
type TxManager struct {
	Err error
}

func (t *TxManager) InTx(ctx context.Context, fn func(context.Context) error) error {
	if t.Err != nil {
		return t.Err
	}
	return fn(ctx)
}
