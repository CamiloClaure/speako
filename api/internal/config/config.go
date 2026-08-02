package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is every knob the process reads from the environment, parsed once at
// boot. A missing required value aborts startup rather than failing later on
// the first request that needs it.
type Config struct {
	Env       string
	Addr      string
	DBPath    string
	PublicURL string
	APIURL    string

	SessionSecret string
	SessionTTL    time.Duration
	CookieSecure  bool

	TeacherEmail    string
	TeacherTimezone string
	MeetingURL      string

	MinNotice      time.Duration
	FreeCancel     time.Duration
	SlotRangeLimit time.Duration

	MPAccessToken   string
	MPWebhookSecret string

	R2AccountID string
	R2KeyID     string
	R2Secret    string
	R2Bucket    string

	ResendAPIKey string
	MailFrom     string

	GoogleClientID string
}

// IsProduction reports whether this process is serving real customers, which
// is what decides cookie and TLS strictness.
func (c Config) IsProduction() bool { return c.Env == "production" }

// Load reads and validates the environment. Every problem is collected before
// returning, so a fresh deploy reports all of its missing variables at once
// instead of one per restart.
func Load() (Config, error) {
	var problems []string

	c := Config{
		Env:             envOr("SPEAKO_ENV", "development"),
		Addr:            envOr("SPEAKO_ADDR", ":8080"),
		DBPath:          envOr("SPEAKO_DB_PATH", "./speako.dev.db"),
		PublicURL:       strings.TrimRight(envOr("SPEAKO_PUBLIC_URL", "http://localhost:5173"), "/"),
		APIURL:          strings.TrimRight(envOr("SPEAKO_API_URL", "http://localhost:8080"), "/"),
		SessionSecret:   os.Getenv("SPEAKO_SESSION_SECRET"),
		TeacherEmail:    os.Getenv("SPEAKO_TEACHER_EMAIL"),
		TeacherTimezone: envOr("SPEAKO_TEACHER_TIMEZONE", "America/Sao_Paulo"),
		MeetingURL:      os.Getenv("SPEAKO_MEETING_URL"),
		MPAccessToken:   os.Getenv("MP_ACCESS_TOKEN"),
		MPWebhookSecret: os.Getenv("MP_WEBHOOK_SECRET"),
		R2AccountID:     os.Getenv("R2_ACCOUNT_ID"),
		R2KeyID:         os.Getenv("R2_ACCESS_KEY_ID"),
		R2Secret:        os.Getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:        envOr("R2_BUCKET", "speako-materials"),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		MailFrom:        envOr("SPEAKO_MAIL_FROM", "Speako <ola@speako.com.br>"),
		GoogleClientID:  os.Getenv("GOOGLE_CLIENT_ID"),
	}

	c.SessionTTL = hours("SPEAKO_SESSION_TTL_HOURS", 720, &problems)
	c.MinNotice = hours("SPEAKO_MIN_NOTICE_HOURS", 12, &problems)
	c.FreeCancel = hours("SPEAKO_FREE_CANCEL_HOURS", 24, &problems)
	// Slot queries are capped so a crawler cannot ask for a decade of slots.
	c.SlotRangeLimit = 60 * 24 * time.Hour
	c.CookieSecure = c.IsProduction()

	if _, err := time.LoadLocation(c.TeacherTimezone); err != nil {
		problems = append(problems, fmt.Sprintf("SPEAKO_TEACHER_TIMEZONE %q is not a valid IANA zone", c.TeacherTimezone))
	}

	// Only production insists on the secrets; a developer should be able to run
	// the API and browse the catalogue without a payment account.
	if c.IsProduction() {
		required := map[string]string{
			"SPEAKO_SESSION_SECRET": c.SessionSecret,
			"SPEAKO_MEETING_URL":    c.MeetingURL,
			"MP_ACCESS_TOKEN":       c.MPAccessToken,
			"MP_WEBHOOK_SECRET":     c.MPWebhookSecret,
			"R2_ACCOUNT_ID":         c.R2AccountID,
			"R2_ACCESS_KEY_ID":      c.R2KeyID,
			"R2_SECRET_ACCESS_KEY":  c.R2Secret,
			"RESEND_API_KEY":        c.ResendAPIKey,
		}
		for name, value := range required {
			if value == "" {
				problems = append(problems, name+" is required in production")
			}
		}
		if len(c.SessionSecret) < 32 {
			problems = append(problems, "SPEAKO_SESSION_SECRET must be at least 32 bytes")
		}
		if !strings.HasPrefix(c.PublicURL, "https://") {
			problems = append(problems, "SPEAKO_PUBLIC_URL must be https in production")
		}
	}

	if len(problems) > 0 {
		return Config{}, errors.New("invalid configuration:\n  - " + strings.Join(problems, "\n  - "))
	}
	return c, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// hours reads a whole number of hours, recording a problem rather than
// silently falling back when the value is present but unusable.
func hours(key string, fallback int, problems *[]string) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallback) * time.Hour
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		*problems = append(*problems, fmt.Sprintf("%s must be a non-negative whole number of hours, got %q", key, raw))
		return time.Duration(fallback) * time.Hour
	}
	return time.Duration(n) * time.Hour
}
