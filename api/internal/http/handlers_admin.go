package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
	"github.com/camiloclaure/speako/api/internal/http/dto"
)

// Admin endpoints. Every one of them sits behind requireTeacher, which is the
// real gate - the frontend's route guard is only there to avoid rendering a
// screen the API would refuse anyway.

func (r *Router) handleAdminGetAvailability(w http.ResponseWriter, req *http.Request, _ identity.User) {
	rules, err := r.teacher.ListAvailability(req.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.AvailabilityRule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, dto.AvailabilityRule{
			Weekday:  int(rule.Weekday),
			StartMin: rule.StartMin,
			EndMin:   rule.EndMin,
			SlotMin:  rule.SlotMin,
		})
	}
	writeJSON(w, http.StatusOK, dto.Availability{Rules: out})
}

func (r *Router) handleAdminPutAvailability(w http.ResponseWriter, req *http.Request, _ identity.User) {
	var body dto.Availability
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, validationErrorf("corpo inválido"))
		return
	}
	rules := make([]scheduling.AvailabilityRule, 0, len(body.Rules))
	for _, in := range body.Rules {
		rule, err := scheduling.NewAvailabilityRule(
			time.Weekday(in.Weekday), in.StartMin, in.EndMin, in.SlotMin)
		if err != nil {
			// A malformed window is the teacher's typo, not a server fault.
			writeError(w, validationErrorf("regra inválida para o dia %d", in.Weekday))
			return
		}
		rules = append(rules, rule)
	}
	if err := r.teacher.SetAvailability(req.Context(), rules); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}

func (r *Router) handleAdminBookings(w http.ResponseWriter, req *http.Request, _ identity.User) {
	from, to, err := parseRange(req, 30*24*time.Hour)
	if err != nil {
		writeError(w, err)
		return
	}
	bookings, err := r.teacher.ListBookings(req.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.AdminBooking, 0, len(bookings))
	for _, b := range bookings {
		out = append(out, dto.AdminBooking{
			ID:     string(b.ID),
			Status: string(b.Status),
			Start:  b.Slot.Start.UTC().Format(time.RFC3339),
			End:    b.Slot.End.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, dto.AdminBookingList{Bookings: out})
}

func (r *Router) handleAdminBookingStatus(w http.ResponseWriter, req *http.Request, _ identity.User) {
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, validationErrorf("corpo inválido"))
		return
	}
	id := shared.ID(req.PathValue("id"))
	if err := r.teacher.MarkAttendance(req.Context(), id, scheduling.BookingStatus(body.Status)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}

func (r *Router) handleAdminCancelBooking(w http.ResponseWriter, req *http.Request, _ identity.User) {
	if err := r.teacher.CancelBookingAsTeacher(req.Context(), shared.ID(req.PathValue("id"))); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.Status{Status: "cancelled"})
}

func (r *Router) handleAdminOrders(w http.ResponseWriter, req *http.Request, _ identity.User) {
	orders, err := r.teacher.ListOrders(req.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.Order, 0, len(orders))
	for _, o := range orders {
		out = append(out, dto.Order{
			ID:          string(o.ID),
			Status:      string(o.Status),
			AmountCents: o.Amount.Cents(),
			CreatedAt:   o.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, dto.OrderList{Orders: out})
}

func (r *Router) handleAdminCreateProduct(w http.ResponseWriter, req *http.Request, _ identity.User) {
	var body dto.AdminProductInput
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, validationErrorf("corpo inválido"))
		return
	}
	product, err := catalog.NewProduct(
		shared.ID(body.ID), body.Slug, body.Title, shared.NewMoney(body.PriceCents),
		catalog.Kind(body.Kind), body.FileKey, body.Credits, body.ValidityDays)
	if err != nil {
		writeError(w, validationErrorf("produto inválido: verifique preço, tipo e créditos"))
		return
	}
	if !body.Active {
		product = product.Deactivate()
	}
	if err := r.teacher.UpsertProduct(req.Context(), product); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.Status{Status: "ok"})
}

// parseRange reads from/to query params, defaulting to a window starting now
// and refusing anything longer than max so one request cannot ask for a decade.
func parseRange(req *http.Request, max time.Duration) (time.Time, time.Time, error) {
	q := req.URL.Query()
	now := time.Now().UTC()

	from := now
	if raw := q.Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, validationErrorf("from deve ser uma data RFC3339")
		}
		from = parsed.UTC()
	}
	to := from.Add(max)
	if raw := q.Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, validationErrorf("to deve ser uma data RFC3339")
		}
		to = parsed.UTC()
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, validationErrorf("to deve ser depois de from")
	}
	if to.Sub(from) > max {
		return time.Time{}, time.Time{}, validationErrorf("período máximo de %d dias", int(max.Hours()/24))
	}
	return from, to, nil
}
