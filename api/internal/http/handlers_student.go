package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
	"github.com/camiloclaure/speako/api/internal/http/dto"
)

func (r *Router) handleRequestMagicLink(w http.ResponseWriter, req *http.Request) {
	var body dto.MagicLinkRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := r.auth.RequestMagicLink(req.Context(), body.Email); err != nil {
		// The response is identical whether or not the email exists, and a
		// storage failure must not let a caller tell either way: log and stay
		// 202.
		slog.Error("magic link request failed", "err", err)
	}
	writeJSON(w, http.StatusAccepted, dto.Status{Status: "sent"})
}

func (r *Router) handleVerifyMagicLink(w http.ResponseWriter, req *http.Request) {
	var body dto.VerifyMagicLinkRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Token == "" {
		writeError(w, validationErrorf("token is required"))
		return
	}
	session, err := r.auth.CompleteMagicLink(req.Context(), body.Token)
	if err != nil {
		writeError(w, err)
		return
	}
	r.setSessionCookie(w, session.Token.String())
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}

func (r *Router) handleLogout(w http.ResponseWriter, req *http.Request) {
	if raw := rawTokenFrom(req.Context()); raw != "" {
		_ = r.auth.SignOut(req.Context(), raw)
	}
	r.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}

func (r *Router) handleMe(w http.ResponseWriter, _ *http.Request, user identity.User) {
	writeJSON(w, http.StatusOK, toUserDTO(user))
}

func (r *Router) handleCredits(w http.ResponseWriter, req *http.Request, user identity.User) {
	balance, err := r.booking.GetBalance(req.Context(), user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := dto.Credits{Balance: balance.Balance, ExpiringSoon: balance.ExpiringSoon}
	if !balance.NextExpiry.IsZero() {
		out.NextExpiry = balance.NextExpiry.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, out)
}

func (r *Router) handleOrders(w http.ResponseWriter, req *http.Request, user identity.User) {
	orders, err := r.purchase.ListMyOrders(req.Context(), user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.Order, 0, len(orders))
	for _, o := range orders {
		out = append(out, toOrderDTO(o))
	}
	writeJSON(w, http.StatusOK, dto.OrderList{Orders: out})
}

func (r *Router) handleMaterials(w http.ResponseWriter, req *http.Request, user identity.User) {
	materials, err := r.materials.ListMyMaterials(req.Context(), user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.Material, 0, len(materials))
	for _, m := range materials {
		out = append(out, toMaterialDTO(m))
	}
	writeJSON(w, http.StatusOK, dto.MaterialList{Materials: out})
}

func (r *Router) handleMaterialDownload(w http.ResponseWriter, req *http.Request, user identity.User) {
	url, err := r.materials.IssueDownloadURL(req.Context(), user.ID, req.PathValue("slug"))
	if err != nil {
		writeError(w, err)
		return
	}
	http.Redirect(w, req, url, http.StatusFound)
}

func (r *Router) handleBookings(w http.ResponseWriter, req *http.Request, user identity.User) {
	bookings, err := r.booking.ListMyBookings(req.Context(), user.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.Booking, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, toBookingDTO(b))
	}
	writeJSON(w, http.StatusOK, dto.BookingList{Bookings: out})
}

func (r *Router) handleCheckout(w http.ResponseWriter, req *http.Request, user identity.User) {
	var body dto.CheckoutRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.Quantity < 1 {
		writeError(w, validationErrorf("quantity must be at least 1"))
		return
	}
	url, orderID, err := r.purchase.StartCheckout(req.Context(), user.ID, body.ProductSlug, body.Quantity)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.CheckoutResponse{OrderID: orderID.String(), CheckoutURL: url})
}

func (r *Router) handleBookLesson(w http.ResponseWriter, req *http.Request, user identity.User) {
	var body dto.BookingRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	start, err := time.Parse(time.RFC3339, body.StartsAt)
	if err != nil {
		writeError(w, validationErrorf("starts_at must be an RFC3339 timestamp"))
		return
	}
	if err := r.booking.BookLesson(req.Context(), user.ID, start); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.Status{Status: "booked"})
}

func (r *Router) handleCancelBooking(w http.ResponseWriter, req *http.Request, user identity.User) {
	id := req.PathValue("id")
	if id == "" {
		writeError(w, validationErrorf("booking id is required"))
		return
	}
	if err := r.booking.CancelBooking(req.Context(), user.ID, shared.ID(id)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.Status{Status: "cancelled"})
}

func toUserDTO(u identity.User) dto.User {
	return dto.User{
		ID:       u.ID.String(),
		Email:    u.Email.String(),
		Name:     u.Name,
		Role:     string(u.Role),
		Timezone: u.Timezone,
	}
}

func toOrderDTO(o ordering.Order) dto.Order {
	d := dto.Order{
		ID:          o.ID.String(),
		Status:      string(o.Status),
		AmountCents: o.Amount.Cents(),
		CreatedAt:   o.CreatedAt.UTC().Format(time.RFC3339),
		ProviderRef: o.ProviderRef,
	}
	if !o.PaidAt.IsZero() {
		d.PaidAt = o.PaidAt.UTC().Format(time.RFC3339)
	}
	return d
}

func toMaterialDTO(p catalog.Product) dto.Material {
	return dto.Material{ID: p.ID.String(), Slug: p.Slug, Title: p.Title}
}

func toBookingDTO(b scheduling.Booking) dto.Booking {
	return dto.Booking{
		ID:     b.ID.String(),
		Status: string(b.Status),
		Start:  b.Slot.Start.UTC().Format(time.RFC3339),
		End:    b.Slot.End.UTC().Format(time.RFC3339),
	}
}
