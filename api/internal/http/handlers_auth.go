package http

import (
	"net/http"

	"github.com/camiloclaure/speako/api/internal/http/dto"
)

func (r *Router) handleRegister(w http.ResponseWriter, req *http.Request) {
	var body dto.RegisterRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	session, err := r.auth.RegisterWithPassword(req.Context(), body.Email, body.Name, body.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	r.setSessionCookie(w, session.Token.String())
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}

// handleLogin must respond identically - same status, same body, byte for
// byte - whether the email is unknown, the account has no password set, or
// the password is wrong. That guarantee lives in app.SignInWithPassword
// (always the same ErrInvalidCredentials sentinel); this handler just must
// not add a second opinion on top of it (no extra logging of which case,
// no different message).
func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	var body dto.LoginRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	session, err := r.auth.SignInWithPassword(req.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	r.setSessionCookie(w, session.Token.String())
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}
