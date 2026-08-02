package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/http/dto"
)

const sessionCookieName = "speako_session"

type contextKey string

const (
	ctxKeyUser      contextKey = "speako.user"
	ctxKeyRawToken  contextKey = "speako.raw_token"
	ctxKeyRequestID contextKey = "speako.request_id"
)

func userFrom(ctx context.Context) (identity.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(identity.User)
	return u, ok
}

func rawTokenFrom(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyRawToken).(string)
	return s
}

func reqIDFrom(ctx context.Context) string {
	s, _ := ctx.Value(ctxKeyRequestID).(string)
	return s
}

// chain composes the middleware in order: request id, panic recovery, access
// log, CORS and session resolution.
func (r *Router) chain(next http.Handler) http.Handler {
	return r.requestID(r.recoverPanic(r.accessLog(r.cors(r.session(next)))))
}

func (r *Router) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := reqIDFrom(req.Context())
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(req.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

func (r *Router) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				slog.Error("panic recovered", "panic", p, "request_id", reqIDFrom(req.Context()))
				writeJSON(w, http.StatusInternalServerError, dto.ErrorEnvelope{
					Error: dto.ErrorBody{Code: string(codeInternal), Message: "internal error"},
				})
			}
		}()
		next.ServeHTTP(w, req)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	if sr.status == 0 {
		sr.status = code
	}
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}

// accessLog emits one structured line per request. It records no PII: no
// headers, no emails, no tokens, no bodies, no query strings.
func (r *Router) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, req)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		slog.Info("http",
			"method", req.Method,
			"path", req.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", reqIDFrom(req.Context()),
		)
	})
}

// cors allows only the configured origin, and only with credentials, so the
// session cookie is never readable from a foreign page.
func (r *Router) cors(next http.Handler) http.Handler {
	allowOrigin := strings.TrimRight(r.config.PublicURL, "/")
	if allowOrigin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		from := req.Header.Get("Origin")
		if from == allowOrigin {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if req.Method == http.MethodOptions {
			if from != allowOrigin {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// session resolves the session cookie best-effort and puts the user and the
// raw token in the request context. A missing or dead session is not an error
// here: guarded handlers reject the request themselves via requireUser.
func (r *Router) session(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie(sessionCookieName)
		if err != nil {
			next.ServeHTTP(w, req)
			return
		}
		user, err := r.auth.GetMe(req.Context(), cookie.Value)
		if err != nil {
			next.ServeHTTP(w, req)
			return
		}
		ctx := context.WithValue(req.Context(), ctxKeyUser, user)
		ctx = context.WithValue(ctx, ctxKeyRawToken, cookie.Value)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

// setSessionCookie writes the session cookie the browser keeps. Only the raw
// token travels; the database stores its hash.
func (r *Router) setSessionCookie(w http.ResponseWriter, raw string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(r.config.SessionTTL.Seconds()),
	})
}

func (r *Router) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// requireUser guards a handler behind a live session.
func (r *Router) requireUser(next func(http.ResponseWriter, *http.Request, identity.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		user, ok := userFrom(req.Context())
		if !ok {
			writeError(w, errUnauthenticated)
			return
		}
		next(w, req, user)
	}
}

// requireTeacher gates the admin surface. This is the real check - the SPA's
// route guard only avoids rendering a screen the API would refuse anyway, and
// a student who types /admin must be turned away here, not there.
func (r *Router) requireTeacher(next func(http.ResponseWriter, *http.Request, identity.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		user, ok := userFrom(req.Context())
		if !ok {
			writeError(w, errUnauthenticated)
			return
		}
		if !user.Role.IsTeacher() && !user.Role.IsAdmin() {
			writeError(w, errForbidden)
			return
		}
		next(w, req, user)
	}
}
