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

// handleGoogleSignIn never treats the request body's claims as trusted -
// SignInWithGoogle verifies the id_token itself and returns the identity the
// token attests to; this handler only rejects the one thing it can check
// without asking the use case: an empty token.
func (r *Router) handleGoogleSignIn(w http.ResponseWriter, req *http.Request) {
	var body dto.GoogleSignInRequest
	if err := decodeJSON(req, &body); err != nil {
		writeError(w, err)
		return
	}
	if body.IDToken == "" {
		writeError(w, validationErrorf("id_token is required"))
		return
	}
	session, err := r.auth.SignInWithGoogle(req.Context(), body.IDToken)
	if err != nil {
		writeError(w, err)
		return
	}
	r.setSessionCookie(w, session.Token.String())
	writeJSON(w, http.StatusOK, dto.Status{Status: "ok"})
}
