package http

import (
	"log/slog"
	"net/http"

	"github.com/camiloclaure/speako/api/internal/http/dto"
)

func (r *Router) handleMercadoPagoWebhook(w http.ResponseWriter, req *http.Request) {
	// ponytail: real processing (x-signature verification against the
	// webhook secret, payment status resolution through the mercadopago
	// adapter and the idempotent grant) lands in S6. Until then the
	// notification is acknowledged so the gateway stops retrying; nothing is
	// written and no amount is trusted.
	var notification dto.MercadoPagoWebhook
	if err := decodeJSON(req, &notification); err != nil {
		writeError(w, err)
		return
	}
	slog.Info("mercadopago webhook acknowledged", "type", notification.Type)
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}
