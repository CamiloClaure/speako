package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

// conn carries the pool and the query routing shared by every repo.
type conn struct {
	db *sql.DB
}

// q returns the transaction from ctx when one is present, the pool otherwise,
// so repo calls made inside TxManager.InTx join that transaction.
func (c *conn) q(ctx context.Context) querier {
	if tx := txFrom(ctx); tx != nil {
		return tx
	}
	return c.db
}

// run executes fn atomically: inside the ctx transaction if present, otherwise
// in a fresh transaction.
func (c *conn) run(ctx context.Context, fn func(context.Context) error) error {
	if txFrom(ctx) != nil {
		return fn(ctx)
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(WithTx(ctx, tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func mapErr(what string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", what, ports.ErrNotFound)
	}
	return err
}

func fromUnix(v int64) time.Time { return time.Unix(v, 0).UTC() }

// optUnix stores a time as unix seconds, or NULL for the zero time.
func optUnix(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.Unix()
}

func optStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func strOrEmpty(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// UserRepo
// ---------------------------------------------------------------------------

// UserRepo implements ports.UserRepo on SQLite.
type UserRepo struct{ conn }

// NewUserRepo returns a UserRepo backed by db.
func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{conn{db: db}} }

var _ ports.UserRepo = (*UserRepo)(nil)

func (r *UserRepo) Create(ctx context.Context, u identity.User) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO users (id, email, name, role, timezone) VALUES (?, ?, ?, ?, ?)`,
		string(u.ID), string(u.Email), u.Name, string(u.Role), u.Timezone)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id shared.ID) (identity.User, error) {
	return r.getUser(ctx, `SELECT id, email, name, role, timezone FROM users WHERE id = ?`, string(id))
}

func (r *UserRepo) GetByEmail(ctx context.Context, email shared.EmailAddress) (identity.User, error) {
	return r.getUser(ctx, `SELECT id, email, name, role, timezone FROM users WHERE email = ?`, string(email))
}

func (r *UserRepo) getUser(ctx context.Context, query string, arg any) (identity.User, error) {
	var u identity.User
	var id, email, role string
	err := r.q(ctx).QueryRowContext(ctx, query, arg).Scan(&id, &email, &u.Name, &role, &u.Timezone)
	if err != nil {
		return identity.User{}, mapErr("get user", err)
	}
	u.ID = shared.ID(id)
	u.Email = shared.EmailAddress(email)
	u.Role = identity.Role(role)
	return u, nil
}

// ---------------------------------------------------------------------------
// SessionRepo
// ---------------------------------------------------------------------------

// SessionRepo implements ports.SessionRepo on SQLite.
type SessionRepo struct{ conn }

// NewSessionRepo returns a SessionRepo backed by db.
func NewSessionRepo(db *sql.DB) *SessionRepo { return &SessionRepo{conn{db: db}} }

var _ ports.SessionRepo = (*SessionRepo)(nil)

func (r *SessionRepo) Create(ctx context.Context, s identity.Session) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		string(s.Token), string(s.UserID), s.ExpiresAt.Unix())
	return err
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash shared.ID) (identity.Session, error) {
	var s identity.Session
	var token, userID string
	var expiresAt int64
	err := r.q(ctx).QueryRowContext(ctx,
		`SELECT token_hash, user_id, expires_at FROM sessions WHERE token_hash = ?`,
		string(tokenHash)).Scan(&token, &userID, &expiresAt)
	if err != nil {
		return identity.Session{}, mapErr("get session", err)
	}
	s.Token = shared.ID(token)
	s.UserID = shared.ID(userID)
	s.ExpiresAt = fromUnix(expiresAt)
	return s, nil
}

func (r *SessionRepo) Delete(ctx context.Context, tokenHash shared.ID) error {
	_, err := r.q(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, string(tokenHash))
	return err
}

func (r *SessionRepo) DeleteExpired(ctx context.Context, before time.Time) error {
	_, err := r.q(ctx).ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, before.Unix())
	return err
}

// ---------------------------------------------------------------------------
// LoginTokenRepo
// ---------------------------------------------------------------------------

// LoginTokenRepo implements ports.LoginTokenRepo on SQLite.
type LoginTokenRepo struct{ conn }

// NewLoginTokenRepo returns a LoginTokenRepo backed by db.
func NewLoginTokenRepo(db *sql.DB) *LoginTokenRepo { return &LoginTokenRepo{conn{db: db}} }

var _ ports.LoginTokenRepo = (*LoginTokenRepo)(nil)

func (r *LoginTokenRepo) Create(ctx context.Context, t ports.LoginToken) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO login_tokens (token_hash, email, expires_at, used_at) VALUES (?, ?, ?, ?)`,
		t.TokenHash, string(t.Email), t.ExpiresAt.Unix(), optUnix(t.UsedAt))
	return err
}

// Consume marks an unused, unexpired token as used, atomically. Any other
// outcome - unknown, already used, or expired - reports ports.ErrNotFound, so
// a forwarded magic link can never log in twice.
func (r *LoginTokenRepo) Consume(ctx context.Context, tokenHash string, now time.Time) (ports.LoginToken, error) {
	q := r.q(ctx)
	res, err := q.ExecContext(ctx,
		`UPDATE login_tokens SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		now.Unix(), tokenHash, now.Unix())
	if err != nil {
		return ports.LoginToken{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return ports.LoginToken{}, err
	}
	if n == 0 {
		return ports.LoginToken{}, fmt.Errorf("consume login token: %w", ports.ErrNotFound)
	}
	var t ports.LoginToken
	var email string
	var expiresAt, usedAt int64
	err = q.QueryRowContext(ctx,
		`SELECT token_hash, email, expires_at, used_at FROM login_tokens WHERE token_hash = ?`,
		tokenHash).Scan(&t.TokenHash, &email, &expiresAt, &usedAt)
	if err != nil {
		return ports.LoginToken{}, mapErr("consume login token", err)
	}
	t.Email = shared.EmailAddress(email)
	t.ExpiresAt = fromUnix(expiresAt)
	t.UsedAt = fromUnix(usedAt)
	return t, nil
}

func (r *LoginTokenRepo) DeleteExpired(ctx context.Context, before time.Time) error {
	_, err := r.q(ctx).ExecContext(ctx, `DELETE FROM login_tokens WHERE expires_at < ?`, before.Unix())
	return err
}

// ---------------------------------------------------------------------------
// ProductRepo
// ---------------------------------------------------------------------------

// ProductRepo implements ports.ProductRepo on SQLite.
type ProductRepo struct{ conn }

// NewProductRepo returns a ProductRepo backed by db.
func NewProductRepo(db *sql.DB) *ProductRepo { return &ProductRepo{conn{db: db}} }

var _ ports.ProductRepo = (*ProductRepo)(nil)

func (r *ProductRepo) Create(ctx context.Context, p catalog.Product) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO products (id, slug, title, kind, price_cents, file_key, credits, validity_days, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(p.ID), p.Slug, p.Title, string(p.Kind), int64(p.Price), p.FileKey, p.Credits,
		p.ValidityDays, boolInt(p.Active))
	return err
}

func (r *ProductRepo) Update(ctx context.Context, p catalog.Product) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO products (id, slug, title, kind, price_cents, file_key, credits, validity_days, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		   slug = excluded.slug,
		   title = excluded.title,
		   kind = excluded.kind,
		   price_cents = excluded.price_cents,
		   file_key = excluded.file_key,
		   credits = excluded.credits,
		   validity_days = excluded.validity_days,
		   active = excluded.active`,
		string(p.ID), p.Slug, p.Title, string(p.Kind), int64(p.Price), p.FileKey, p.Credits,
		p.ValidityDays, boolInt(p.Active))
	return err
}

func (r *ProductRepo) GetByID(ctx context.Context, id shared.ID) (catalog.Product, error) {
	return r.getProduct(ctx, `SELECT id, slug, title, kind, price_cents, file_key, credits, validity_days, active FROM products WHERE id = ?`, string(id))
}

func (r *ProductRepo) GetBySlug(ctx context.Context, slug string) (catalog.Product, error) {
	return r.getProduct(ctx, `SELECT id, slug, title, kind, price_cents, file_key, credits, validity_days, active FROM products WHERE slug = ?`, slug)
}

func (r *ProductRepo) List(ctx context.Context) ([]catalog.Product, error) {
	rows, err := r.q(ctx).QueryContext(ctx, `SELECT id, slug, title, kind, price_cents, file_key, credits, validity_days, active FROM products ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []catalog.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *ProductRepo) getProduct(ctx context.Context, query string, arg any) (catalog.Product, error) {
	p, err := scanProduct(r.q(ctx).QueryRowContext(ctx, query, arg))
	if err != nil {
		return catalog.Product{}, mapErr("get product", err)
	}
	return p, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProduct(sc rowScanner) (catalog.Product, error) {
	var p catalog.Product
	var id, slug, title, kind string
	var price int64
	var active int
	err := sc.Scan(&id, &slug, &title, &kind, &price, &p.FileKey, &p.Credits, &p.ValidityDays, &active)
	if err != nil {
		return catalog.Product{}, err
	}
	p.ID = shared.ID(id)
	p.Slug = slug
	p.Title = title
	p.Kind = catalog.Kind(kind)
	p.Price = shared.Money(price)
	p.Active = active != 0
	return p, nil
}

// ---------------------------------------------------------------------------
// OrderRepo
// ---------------------------------------------------------------------------

// OrderRepo implements ports.OrderRepo on SQLite.
type OrderRepo struct{ conn }

// NewOrderRepo returns an OrderRepo backed by db.
func NewOrderRepo(db *sql.DB) *OrderRepo { return &OrderRepo{conn{db: db}} }

var _ ports.OrderRepo = (*OrderRepo)(nil)

func (r *OrderRepo) Create(ctx context.Context, o ordering.Order) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO orders (id, user_id, product_id, status, amount_cents, provider_ref, created_at, paid_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		string(o.ID), string(o.UserID), string(o.ProductID), string(o.Status), int64(o.Amount),
		optStr(o.ProviderRef), o.CreatedAt.Unix(), optUnix(o.PaidAt))
	return err
}

func (r *OrderRepo) Save(ctx context.Context, o ordering.Order) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO orders (id, user_id, product_id, status, amount_cents, provider_ref, created_at, paid_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		   user_id = excluded.user_id,
		   product_id = excluded.product_id,
		   status = excluded.status,
		   amount_cents = excluded.amount_cents,
		   provider_ref = excluded.provider_ref,
		   paid_at = excluded.paid_at`,
		string(o.ID), string(o.UserID), string(o.ProductID), string(o.Status), int64(o.Amount),
		optStr(o.ProviderRef), o.CreatedAt.Unix(), optUnix(o.PaidAt))
	return err
}

func (r *OrderRepo) GetByID(ctx context.Context, id shared.ID) (ordering.Order, error) {
	o, err := scanOrder(r.q(ctx).QueryRowContext(ctx,
		`SELECT id, user_id, product_id, status, amount_cents, provider_ref, created_at, paid_at FROM orders WHERE id = ?`,
		string(id)))
	if err != nil {
		return ordering.Order{}, mapErr("get order", err)
	}
	return o, nil
}

func (r *OrderRepo) ListByUserID(ctx context.Context, userID shared.ID) ([]ordering.Order, error) {
	return r.listOrders(ctx,
		`SELECT id, user_id, product_id, status, amount_cents, provider_ref, created_at, paid_at FROM orders WHERE user_id = ? ORDER BY created_at, rowid`,
		string(userID))
}

func (r *OrderRepo) ListPendingBefore(ctx context.Context, before time.Time) ([]ordering.Order, error) {
	return r.listOrders(ctx,
		`SELECT id, user_id, product_id, status, amount_cents, provider_ref, created_at, paid_at FROM orders WHERE status = 'pending' AND created_at < ? ORDER BY created_at, rowid`,
		before.Unix())
}

func (r *OrderRepo) listOrders(ctx context.Context, query string, arg any) ([]ordering.Order, error) {
	rows, err := r.q(ctx).QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ordering.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func scanOrder(sc rowScanner) (ordering.Order, error) {
	var o ordering.Order
	var id, userID, productID, status string
	var amount int64
	var providerRef sql.NullString
	var createdAt int64
	var paidAt sql.NullInt64
	err := sc.Scan(&id, &userID, &productID, &status, &amount, &providerRef, &createdAt, &paidAt)
	if err != nil {
		return ordering.Order{}, err
	}
	o.ID = shared.ID(id)
	o.UserID = shared.ID(userID)
	o.ProductID = shared.ID(productID)
	o.Status = ordering.Status(status)
	o.Amount = shared.Money(amount)
	o.ProviderRef = strOrEmpty(providerRef)
	o.CreatedAt = fromUnix(createdAt)
	if paidAt.Valid {
		o.PaidAt = fromUnix(paidAt.Int64)
	}
	return o, nil
}

// ---------------------------------------------------------------------------
// PaymentEventRepo
// ---------------------------------------------------------------------------

// PaymentEventRepo implements ports.PaymentEventRepo on SQLite.
type PaymentEventRepo struct{ conn }

// NewPaymentEventRepo returns a PaymentEventRepo backed by db.
func NewPaymentEventRepo(db *sql.DB) *PaymentEventRepo { return &PaymentEventRepo{conn{db: db}} }

var _ ports.PaymentEventRepo = (*PaymentEventRepo)(nil)

// Claim records eventID as handled and reports whether this caller won the
// race. A second call for the same eventID trips the UNIQUE constraint and
// reports false, which is what makes webhook retries idempotent.
func (r *PaymentEventRepo) Claim(ctx context.Context, eventID string, orderID shared.ID, now time.Time) (bool, error) {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO payment_events (event_id, order_id, processed_at) VALUES (?, ?, ?)`,
		eventID, string(orderID), now.Unix())
	if err == nil {
		return true, nil
	}
	if isUniqueViolation(err) {
		return false, nil
	}
	return false, err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// ---------------------------------------------------------------------------
// EntitlementRepo
// ---------------------------------------------------------------------------

// EntitlementRepo implements ports.EntitlementRepo on SQLite.
type EntitlementRepo struct{ conn }

// NewEntitlementRepo returns an EntitlementRepo backed by db.
func NewEntitlementRepo(db *sql.DB) *EntitlementRepo { return &EntitlementRepo{conn{db: db}} }

var _ ports.EntitlementRepo = (*EntitlementRepo)(nil)

func (r *EntitlementRepo) Add(ctx context.Context, e access.Entitlement) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO entitlements (user_id, product_id, granted_at) VALUES (?, ?, ?)`,
		string(e.UserID), string(e.ProductID), e.GrantedAt.Unix())
	return err
}

func (r *EntitlementRepo) ListByUserID(ctx context.Context, userID shared.ID) ([]access.Entitlement, error) {
	rows, err := r.q(ctx).QueryContext(ctx,
		`SELECT user_id, product_id, granted_at FROM entitlements WHERE user_id = ? ORDER BY granted_at, rowid`,
		string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []access.Entitlement
	for rows.Next() {
		var e access.Entitlement
		var uid, pid string
		var grantedAt int64
		if err := rows.Scan(&uid, &pid, &grantedAt); err != nil {
			return nil, err
		}
		e.UserID = shared.ID(uid)
		e.ProductID = shared.ID(pid)
		e.GrantedAt = fromUnix(grantedAt)
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// CreditLedgerRepo
// ---------------------------------------------------------------------------

// CreditLedgerRepo implements ports.CreditLedgerRepo on SQLite.
type CreditLedgerRepo struct{ conn }

// NewCreditLedgerRepo returns a CreditLedgerRepo backed by db.
func NewCreditLedgerRepo(db *sql.DB) *CreditLedgerRepo { return &CreditLedgerRepo{conn{db: db}} }

var _ ports.CreditLedgerRepo = (*CreditLedgerRepo)(nil)

func (r *CreditLedgerRepo) Append(ctx context.Context, userID shared.ID, e credit.Entry) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO credit_entries (user_id, delta, reason, ref_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		string(userID), e.Delta, e.Reason, e.RefID, optUnix(e.ExpiresAt), time.Now().Unix())
	return err
}

func (r *CreditLedgerRepo) Get(ctx context.Context, userID shared.ID) (credit.Ledger, error) {
	rows, err := r.q(ctx).QueryContext(ctx,
		`SELECT delta, reason, ref_id, expires_at FROM credit_entries WHERE user_id = ? ORDER BY created_at, rowid`,
		string(userID))
	if err != nil {
		return credit.Ledger{}, err
	}
	defer rows.Close()
	var entries []credit.Entry
	for rows.Next() {
		var e credit.Entry
		var expiresAt int64
		if err := rows.Scan(&e.Delta, &e.Reason, &e.RefID, &expiresAt); err != nil {
			return credit.Ledger{}, err
		}
		if expiresAt != 0 {
			e.ExpiresAt = fromUnix(expiresAt)
		}
		entries = append(entries, e)
	}
	return credit.NewLedger(entries...), rows.Err()
}

// ---------------------------------------------------------------------------
// BookingRepo
// ---------------------------------------------------------------------------

// BookingRepo implements ports.BookingRepo on SQLite. The platform runs a
// single teacher, and the domain Booking carries no teacher yet, so the repo
// pins the teacher's id: the schema-level slot uniqueness is enforced by the
// partial unique index on (teacher_id, starts_at), and that index needs a
// value, not NULL.
type BookingRepo struct {
	conn
	teacherID shared.ID
}

// NewBookingRepo returns a BookingRepo backed by db, booking every slot for
// the single teacher identified by teacherID.
func NewBookingRepo(db *sql.DB, teacherID shared.ID) *BookingRepo {
	return &BookingRepo{conn: conn{db: db}, teacherID: teacherID}
}

var _ ports.BookingRepo = (*BookingRepo)(nil)

func (r *BookingRepo) Create(ctx context.Context, b scheduling.Booking) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO bookings (id, student_id, teacher_id, starts_at, ends_at, status) VALUES (?, ?, ?, ?, ?, ?)`,
		string(b.ID), string(b.StudentID), string(r.teacherID), b.Slot.Start.Unix(), b.Slot.End.Unix(), string(b.Status))
	return err
}

func (r *BookingRepo) Save(ctx context.Context, b scheduling.Booking) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO bookings (id, student_id, teacher_id, starts_at, ends_at, status)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET
		   student_id = excluded.student_id,
		   teacher_id = excluded.teacher_id,
		   starts_at = excluded.starts_at,
		   ends_at = excluded.ends_at,
		   status = excluded.status`,
		string(b.ID), string(b.StudentID), string(r.teacherID), b.Slot.Start.Unix(), b.Slot.End.Unix(), string(b.Status))
	return err
}

func (r *BookingRepo) GetByID(ctx context.Context, id shared.ID) (scheduling.Booking, error) {
	b, err := scanBooking(r.q(ctx).QueryRowContext(ctx,
		`SELECT id, student_id, starts_at, ends_at, status FROM bookings WHERE id = ?`,
		string(id)))
	if err != nil {
		return scheduling.Booking{}, mapErr("get booking", err)
	}
	return b, nil
}

func (r *BookingRepo) ListByStudentID(ctx context.Context, studentID shared.ID) ([]scheduling.Booking, error) {
	return r.listBookings(ctx,
		`SELECT id, student_id, starts_at, ends_at, status FROM bookings WHERE student_id = ? ORDER BY starts_at, rowid`,
		string(studentID))
}

// ListBetween returns bookings whose slot overlaps [from, to).
func (r *BookingRepo) ListBetween(ctx context.Context, from, to time.Time) ([]scheduling.Booking, error) {
	return r.listBookings(ctx,
		`SELECT id, student_id, starts_at, ends_at, status FROM bookings WHERE starts_at < ? AND ends_at > ? ORDER BY starts_at, rowid`,
		to.Unix(), from.Unix())
}

func (r *BookingRepo) listBookings(ctx context.Context, query string, args ...any) ([]scheduling.Booking, error) {
	rows, err := r.q(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduling.Booking
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func scanBooking(sc rowScanner) (scheduling.Booking, error) {
	var b scheduling.Booking
	var id, studentID, status string
	var startsAt, endsAt int64
	err := sc.Scan(&id, &studentID, &startsAt, &endsAt, &status)
	if err != nil {
		return scheduling.Booking{}, err
	}
	b.ID = shared.ID(id)
	b.StudentID = shared.ID(studentID)
	b.Slot.Start = fromUnix(startsAt)
	b.Slot.End = fromUnix(endsAt)
	b.Status = scheduling.BookingStatus(status)
	return b, nil
}

// ---------------------------------------------------------------------------
// AvailabilityRepo
// ---------------------------------------------------------------------------

// AvailabilityRepo implements ports.AvailabilityRepo on SQLite.
type AvailabilityRepo struct{ conn }

// NewAvailabilityRepo returns an AvailabilityRepo backed by db.
func NewAvailabilityRepo(db *sql.DB) *AvailabilityRepo { return &AvailabilityRepo{conn{db: db}} }

var _ ports.AvailabilityRepo = (*AvailabilityRepo)(nil)

// SaveRules replaces the whole rule set atomically.
func (r *AvailabilityRepo) SaveRules(ctx context.Context, rules []scheduling.AvailabilityRule) error {
	return r.run(ctx, func(ctx context.Context) error {
		if _, err := r.q(ctx).ExecContext(ctx, `DELETE FROM availability_rules`); err != nil {
			return err
		}
		for _, rule := range rules {
			if _, err := r.q(ctx).ExecContext(ctx,
				`INSERT INTO availability_rules (weekday, start_minute, end_minute, slot_min) VALUES (?, ?, ?, ?)`,
				int(rule.Weekday), rule.StartMin, rule.EndMin, rule.SlotMin); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *AvailabilityRepo) ListRules(ctx context.Context) ([]scheduling.AvailabilityRule, error) {
	rows, err := r.q(ctx).QueryContext(ctx, `SELECT weekday, start_minute, end_minute, slot_min FROM availability_rules ORDER BY weekday, start_minute`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduling.AvailabilityRule
	for rows.Next() {
		var rule scheduling.AvailabilityRule
		var weekday int
		if err := rows.Scan(&weekday, &rule.StartMin, &rule.EndMin, &rule.SlotMin); err != nil {
			return nil, err
		}
		rule.Weekday = time.Weekday(weekday)
		out = append(out, rule)
	}
	return out, rows.Err()
}

func (r *AvailabilityRepo) SaveException(ctx context.Context, e scheduling.Exception) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO availability_exceptions (date, blocked, start_minute, end_minute)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (date) DO UPDATE SET
		   blocked = excluded.blocked,
		   start_minute = excluded.start_minute,
		   end_minute = excluded.end_minute`,
		e.Date.Unix(), boolInt(e.Blocked), e.StartMin, e.EndMin)
	return err
}

func (r *AvailabilityRepo) RemoveException(ctx context.Context, date time.Time) error {
	_, err := r.q(ctx).ExecContext(ctx, `DELETE FROM availability_exceptions WHERE date = ?`, date.Unix())
	return err
}

func (r *AvailabilityRepo) ListExceptions(ctx context.Context) ([]scheduling.Exception, error) {
	rows, err := r.q(ctx).QueryContext(ctx, `SELECT date, blocked, start_minute, end_minute FROM availability_exceptions ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []scheduling.Exception
	for rows.Next() {
		var e scheduling.Exception
		var date int64
		var blocked int
		if err := rows.Scan(&date, &blocked, &e.StartMin, &e.EndMin); err != nil {
			return nil, err
		}
		e.Date = fromUnix(date)
		e.Blocked = blocked != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// LeadRepo
// ---------------------------------------------------------------------------

// LeadRepo implements ports.LeadRepo on SQLite.
type LeadRepo struct{ conn }

// NewLeadRepo returns a LeadRepo backed by db.
func NewLeadRepo(db *sql.DB) *LeadRepo { return &LeadRepo{conn{db: db}} }

var _ ports.LeadRepo = (*LeadRepo)(nil)

func (r *LeadRepo) Create(ctx context.Context, l leads.Lead) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO leads (email, source, consent_at, consent_ip, confirmed_at, confirm_token_hash)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		string(l.Email), l.Source, l.ConsentAt.Unix(), l.ConsentIP, optUnix(l.ConfirmedAt), l.ConfirmTokenHash)
	return err
}

func (r *LeadRepo) Save(ctx context.Context, l leads.Lead) error {
	_, err := r.q(ctx).ExecContext(ctx,
		`INSERT INTO leads (email, source, consent_at, consent_ip, confirmed_at, confirm_token_hash)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (email) DO UPDATE SET
		   source = excluded.source,
		   consent_at = excluded.consent_at,
		   consent_ip = excluded.consent_ip,
		   confirmed_at = excluded.confirmed_at,
		   confirm_token_hash = excluded.confirm_token_hash`,
		string(l.Email), l.Source, l.ConsentAt.Unix(), l.ConsentIP, optUnix(l.ConfirmedAt), l.ConfirmTokenHash)
	return err
}

func (r *LeadRepo) GetByEmail(ctx context.Context, email shared.EmailAddress) (leads.Lead, error) {
	l, err := r.getLead(ctx, `SELECT email, source, consent_at, consent_ip, confirmed_at, confirm_token_hash FROM leads WHERE email = ?`, string(email))
	if err != nil {
		return leads.Lead{}, err
	}
	return l, nil
}

func (r *LeadRepo) GetByConfirmTokenHash(ctx context.Context, hash string) (leads.Lead, error) {
	l, err := r.getLead(ctx, `SELECT email, source, consent_at, consent_ip, confirmed_at, confirm_token_hash FROM leads WHERE confirm_token_hash = ?`, hash)
	if err != nil {
		return leads.Lead{}, err
	}
	return l, nil
}

func (r *LeadRepo) List(ctx context.Context) ([]leads.Lead, error) {
	rows, err := r.q(ctx).QueryContext(ctx, `SELECT email, source, consent_at, consent_ip, confirmed_at, confirm_token_hash FROM leads ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []leads.Lead
	for rows.Next() {
		l, err := scanLead(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (r *LeadRepo) getLead(ctx context.Context, query string, arg any) (leads.Lead, error) {
	l, err := scanLead(r.q(ctx).QueryRowContext(ctx, query, arg))
	if err != nil {
		return leads.Lead{}, mapErr("get lead", err)
	}
	return l, nil
}

func scanLead(sc rowScanner) (leads.Lead, error) {
	var l leads.Lead
	var email, source, consentIP, tokenHash string
	var consentAt int64
	var confirmedAt sql.NullInt64
	err := sc.Scan(&email, &source, &consentAt, &consentIP, &confirmedAt, &tokenHash)
	if err != nil {
		return leads.Lead{}, err
	}
	l.Email = shared.EmailAddress(email)
	l.Source = source
	l.ConsentAt = fromUnix(consentAt)
	l.ConsentIP = consentIP
	l.ConfirmTokenHash = tokenHash
	if confirmedAt.Valid {
		l.ConfirmedAt = fromUnix(confirmedAt.Int64)
	}
	return l, nil
}
