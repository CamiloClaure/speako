package dto

// ErrorBody is one error from the closed set of codes documented in api.yaml.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope wraps every error response.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// Status is the body of endpoints that have nothing to report but a result.
type Status struct {
	Status string `json:"status"`
}

// Health is the liveness response.
type Health struct {
	Status string `json:"status"`
}

// Product is a material or pack on sale. Credits and ValidityDays only apply
// to packs, so they are omitted for materials.
type Product struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Kind         string `json:"kind"`
	PriceCents   int64  `json:"price_cents"`
	Credits      int    `json:"credits,omitempty"`
	ValidityDays int    `json:"validity_days,omitempty"`
	Active       bool   `json:"active"`
}

type ProductList struct {
	Products []Product `json:"products"`
}

// Slot is one bookable lesson window. Times are RFC3339 UTC instants.
type Slot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type SlotList struct {
	Slots []Slot `json:"slots"`
}

// User is the authenticated student as seen by the member area.
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Timezone string `json:"timezone"`
}

// Credits is the student's spendable balance plus a warning about credits
// about to lapse, so a pack does not quietly expire unused.
type Credits struct {
	Balance      int    `json:"balance"`
	ExpiringSoon int    `json:"expiring_soon"`
	NextExpiry   string `json:"next_expiry,omitempty"`
}

// Order is one purchase order and its lifecycle state.
type Order struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AmountCents int64  `json:"amount_cents"`
	CreatedAt   string `json:"created_at"`
	PaidAt      string `json:"paid_at,omitempty"`
	ProviderRef string `json:"provider_ref,omitempty"`
}

type OrderList struct {
	Orders []Order `json:"orders"`
}

// Material is a bought digital file the user may open.
type Material struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type MaterialList struct {
	Materials []Material `json:"materials"`
}

// Booking is one scheduled lesson.
type Booking struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Start  string `json:"start"`
	End    string `json:"end"`
}

type BookingList struct {
	Bookings []Booking `json:"bookings"`
}

type LeadRequest struct {
	Email  string `json:"email"`
	Source string `json:"source"`
}

type MagicLinkRequest struct {
	Email string `json:"email"`
}

type VerifyMagicLinkRequest struct {
	Token string `json:"token"`
}

type CheckoutRequest struct {
	ProductSlug string `json:"product_slug"`
	Quantity    int    `json:"quantity"`
}

type CheckoutResponse struct {
	OrderID     string `json:"order_id"`
	CheckoutURL string `json:"checkout_url"`
}

type BookingRequest struct {
	StartsAt string `json:"starts_at"`
}

// MercadoPagoWebhook is the shape of a Mercado Pago payment notification.
// Only the fields a handler needs are decoded; unknown fields are ignored.
type MercadoPagoWebhook struct {
	Type   string `json:"type"`
	Action string `json:"action"`
	Data   struct {
		ID string `json:"id"`
	} `json:"data"`
}

// AvailabilityRule is one weekly window in the teacher's own timezone.
// Minutes since local midnight, not clock strings, so the client cannot smuggle
// a timezone in.
type AvailabilityRule struct {
	Weekday  int `json:"weekday"`
	StartMin int `json:"start_min"`
	EndMin   int `json:"end_min"`
	SlotMin  int `json:"slot_min"`
}

// Availability is the teacher's whole weekly schedule.
type Availability struct {
	Rules []AvailabilityRule `json:"rules"`
}

// AdminBooking is a booking as the teacher sees it.
type AdminBooking struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Start  string `json:"start"`
	End    string `json:"end"`
}

type AdminBookingList struct {
	Bookings []AdminBooking `json:"bookings"`
}

// AdminProductInput creates or updates a product. The price arrives as integer
// cents; converting from reais is the client's job and is tested there.
type AdminProductInput struct {
	ID           string `json:"id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	PriceCents   int64  `json:"price_cents"`
	Kind         string `json:"kind"`
	FileKey      string `json:"file_key"`
	Credits      int    `json:"credits"`
	ValidityDays int    `json:"validity_days"`
	Active       bool   `json:"active"`
}
