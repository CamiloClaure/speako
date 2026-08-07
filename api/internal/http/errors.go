package http

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/camiloclaure/speako/api/internal/app"
	"github.com/camiloclaure/speako/api/internal/app/ports"
	"github.com/camiloclaure/speako/api/internal/domain/catalog"
	"github.com/camiloclaure/speako/api/internal/domain/credit"
	"github.com/camiloclaure/speako/api/internal/domain/identity"
	"github.com/camiloclaure/speako/api/internal/domain/leads"
	"github.com/camiloclaure/speako/api/internal/domain/ordering"
	"github.com/camiloclaure/speako/api/internal/domain/scheduling"
	"github.com/camiloclaure/speako/api/internal/domain/shared"
	"github.com/camiloclaure/speako/api/internal/http/dto"
)

// errorCode is the closed set of machine-readable error codes the API may
// return. Adding a code changes the contract in api.yaml.
type errorCode string

const (
	codeUnauthenticated     errorCode = "unauthenticated"
	codeForbidden           errorCode = "forbidden"
	codeNotFound            errorCode = "not_found"
	codeValidationFailed    errorCode = "validation_failed"
	codeInsufficientCredits errorCode = "insufficient_credits"
	codeSlotTaken           errorCode = "slot_taken"
	codeTooLateToBook       errorCode = "too_late_to_book"
	codeTooLateToCancel     errorCode = "too_late_to_cancel"
	codePaymentFailed       errorCode = "payment_failed"
	codeRateLimited         errorCode = "rate_limited"
	codeInvalidCredentials  errorCode = "invalid_credentials"
	codeEmailTaken          errorCode = "email_taken"
	codeInternal            errorCode = "internal"
	codeNotImplemented      errorCode = "not_implemented"
)

// validationErrors are the domain sentinels that describe a caller mistake,
// as opposed to a missing resource or a state conflict.
var validationErrors = []error{
	shared.ErrInvalidEmail,
	shared.ErrInvalidTimeSlot,
	shared.ErrInvalidMoney,
	shared.ErrQuantityTooSmall,
	identity.ErrInvalidUser,
	identity.ErrInvalidRole,
	identity.ErrInvalidTimezone,
	identity.ErrEmptyName,
	identity.ErrWeakPassword,
	identity.ErrPasswordTooLong,
	catalog.ErrInvalidProduct,
	credit.ErrInvalidDebit,
	leads.ErrMissingConsentIP,
	leads.ErrMissingConsentTime,
	scheduling.ErrInvalidRule,
	scheduling.ErrInvalidException,
	ordering.ErrIllegalTransition,
}

// writeError is the single place that maps a domain error to a status code
// and an error code from the closed set, then writes the envelope. Unknown
// errors become 500 internal: their detail is logged here and never reaches
// the body.
func writeError(w http.ResponseWriter, err error) {
	var (
		status  int
		code    errorCode
		message string
	)
	switch {
	case errors.Is(err, errUnauthenticated):
		status, code, message = http.StatusUnauthorized, codeUnauthenticated, "authentication required"
	case errors.Is(err, app.ErrSessionExpired), errors.Is(err, identity.ErrInvalidToken):
		status, code, message = http.StatusUnauthorized, codeUnauthenticated, err.Error()
	case errors.Is(err, app.ErrInvalidCredentials):
		// Deliberately the same status, code and message every time this
		// sentinel is returned - see app.ErrInvalidCredentials's doc comment.
		status, code, message = http.StatusUnauthorized, codeInvalidCredentials, err.Error()
	case errors.Is(err, errForbidden):
		status, code, message = http.StatusForbidden, codeForbidden, err.Error()
	case errors.Is(err, app.ErrNoEntitlement):
		status, code, message = http.StatusForbidden, codeForbidden, err.Error()
	case errors.Is(err, errRateLimited):
		status, code, message = http.StatusTooManyRequests, codeRateLimited, err.Error()
	case errors.Is(err, ports.ErrNotFound):
		status, code, message = http.StatusNotFound, codeNotFound, "not found"
	case errors.Is(err, credit.ErrInsufficientCredits):
		status, code, message = http.StatusConflict, codeInsufficientCredits, err.Error()
	case errors.Is(err, app.ErrEmailTaken):
		status, code, message = http.StatusConflict, codeEmailTaken, err.Error()
	case errors.Is(err, scheduling.ErrSlotTaken):
		status, code, message = http.StatusConflict, codeSlotTaken, err.Error()
	case errors.Is(err, scheduling.ErrTooLateToBook):
		status, code, message = http.StatusUnprocessableEntity, codeTooLateToBook, err.Error()
	case errors.Is(err, app.ErrSlotUnavailable):
		// Not the same as slot_taken: the requested time is not on the
		// teacher's calendar at all. Reporting it as "taken" sends a student
		// hunting for another time when the real answer is "that is not a slot".
		status, code, message = http.StatusUnprocessableEntity, codeValidationFailed,
			"that time is not an available slot"
	case errors.Is(err, errNotImplemented):
		status, code, message = http.StatusNotImplemented, codeNotImplemented, "not implemented"
	case isValidation(err):
		status, code, message = http.StatusUnprocessableEntity, codeValidationFailed, err.Error()
	default:
		slog.Error("internal error", "err", err)
		status, code, message = http.StatusInternalServerError, codeInternal, "internal error"
	}
	writeJSON(w, status, dto.ErrorEnvelope{
		Error: dto.ErrorBody{Code: string(code), Message: message},
	})
}

// isValidation reports whether err describes a caller mistake. Transport-level
// validation uses a shared.DomainError with the validation_failed kind, so
// both that and the domain sentinels are accepted here.
func isValidation(err error) bool {
	for _, e := range validationErrors {
		if errors.Is(err, e) {
			return true
		}
	}
	var de shared.DomainError
	return errors.As(err, &de) && de.Kind() == "validation_failed"
}
