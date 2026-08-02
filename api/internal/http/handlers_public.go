package http

import (
	"net"
	"net/http"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
	"github.com/camiloclaure/speako/api/internal/http/dto"
)

func (r *Router) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dto.Health{Status: "ok"})
}

func (r *Router) handleListProducts(w http.ResponseWriter, req *http.Request) {
	products, err := r.catalog.ListProducts(req.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.Product, 0, len(products))
	for _, p := range products {
		out = append(out, toProductDTO(p))
	}
	writeJSON(w, http.StatusOK, dto.ProductList{Products: out})
}

func (r *Router) handleGetProduct(w http.ResponseWriter, req *http.Request) {
	product, err := r.catalog.GetProduct(req.Context(), req.PathValue("slug"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toProductDTO(product))
}

func (r *Router) handleListSlots(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	from, err := time.Parse(time.RFC3339, q.Get("from"))
	if err != nil {
		writeError(w, validationErrorf("from must be an RFC3339 timestamp"))
		return
	}
	to, err := time.Parse(time.RFC3339, q.Get("to"))
	if err != nil {
		writeError(w, validationErrorf("to must be an RFC3339 timestamp"))
		return
	}
	if !to.After(from) {
		writeError(w, validationErrorf("to must be after from"))
		return
	}
	tz := q.Get("tz")
	if tz == "" {
		writeError(w, validationErrorf("tz is required"))
		return
	}
	if _, err := time.LoadLocation(tz); err != nil {
		writeError(w, validationErrorf("tz must be an IANA timezone"))
		return
	}
	slots, err := r.booking.ListAvailableSlots(req.Context(), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]dto.Slot, 0, len(slots))
	for _, s := range slots {
		out = append(out, toSlotDTO(s))
	}
	writeJSON(w, http.StatusOK, dto.SlotList{Slots: out})
}

func (r *Router) handleCaptureLead(w http.ResponseWriter, req *http.Request) {
	var body dto.LeadRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	// The consent IP is evidence of the consent event, so it comes from the
	// connection, never from a header the visitor controls.
	if err := r.leads.CaptureLead(req.Context(), body.Email, body.Source, requestIP(req)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dto.Status{Status: "captured"})
}

func (r *Router) handleConfirmLead(w http.ResponseWriter, req *http.Request) {
	token := req.URL.Query().Get("token")
	if token == "" {
		writeError(w, validationErrorf("token is required"))
		return
	}
	if err := r.leads.ConfirmLead(req.Context(), token); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto.Status{Status: "confirmed"})
}

func requestIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func toProductDTO(p catalog.Product) dto.Product {
	return dto.Product{
		ID:           p.ID.String(),
		Slug:         p.Slug,
		Title:        p.Title,
		Kind:         string(p.Kind),
		PriceCents:   p.Price.Cents(),
		Credits:      p.Credits,
		ValidityDays: p.ValidityDays,
		Active:       p.Active,
	}
}

func toSlotDTO(s shared.TimeSlot) dto.Slot {
	return dto.Slot{
		Start: s.Start.UTC().Format(time.RFC3339),
		End:   s.End.UTC().Format(time.RFC3339),
	}
}
