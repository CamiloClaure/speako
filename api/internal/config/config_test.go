package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaultsInDevelopment(t *testing.T) {
	t.Setenv("SPEAKO_ENV", "development")

	c, err := Load()
	if err != nil {
		t.Fatalf("development config must load without secrets: %v", err)
	}
	if c.MinNotice != 12*time.Hour || c.FreeCancel != 24*time.Hour {
		t.Errorf("policy defaults = %v/%v, want 12h/24h", c.MinNotice, c.FreeCancel)
	}
	if c.CookieSecure {
		t.Error("Secure cookies in development would break http://localhost")
	}
}

// Production must refuse to start rather than serve with a missing secret, and
// it must report every problem at once - one-per-restart is how a deploy loop
// burns an afternoon.
func TestLoadRejectsIncompleteProduction(t *testing.T) {
	t.Setenv("SPEAKO_ENV", "production")
	t.Setenv("SPEAKO_PUBLIC_URL", "http://speako.com.br")
	t.Setenv("SPEAKO_SESSION_SECRET", "too-short")

	_, err := Load()
	if err == nil {
		t.Fatal("production loaded with no payment or mail credentials")
	}
	msg := err.Error()
	for _, want := range []string{
		"MP_ACCESS_TOKEN",
		"RESEND_API_KEY",
		"at least 32 bytes",
		"must be https",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q:\n%s", want, msg)
		}
	}
}

func TestLoadRejectsUnusableValues(t *testing.T) {
	t.Setenv("SPEAKO_ENV", "development")
	t.Setenv("SPEAKO_MIN_NOTICE_HOURS", "soon")
	t.Setenv("SPEAKO_TEACHER_TIMEZONE", "Mars/Olympus")

	_, err := Load()
	if err == nil {
		t.Fatal("a nonsense timezone and duration were accepted")
	}
	if !strings.Contains(err.Error(), "SPEAKO_MIN_NOTICE_HOURS") {
		t.Errorf("unparseable duration not reported: %v", err)
	}
	if !strings.Contains(err.Error(), "Mars/Olympus") {
		t.Errorf("invalid timezone not reported: %v", err)
	}
}

func TestCookieSecureFollowsEnvironment(t *testing.T) {
	t.Setenv("SPEAKO_ENV", "production")
	t.Setenv("SPEAKO_PUBLIC_URL", "https://speako.com.br")
	t.Setenv("SPEAKO_SESSION_SECRET", strings.Repeat("x", 32))
	t.Setenv("SPEAKO_MEETING_URL", "https://meet.google.com/abc-defg-hij")
	for _, k := range []string{"MP_ACCESS_TOKEN", "MP_WEBHOOK_SECRET", "R2_ACCOUNT_ID", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "RESEND_API_KEY"} {
		t.Setenv(k, "set")
	}

	c, err := Load()
	if err != nil {
		t.Fatalf("complete production config rejected: %v", err)
	}
	if !c.CookieSecure {
		t.Error("production must set Secure on the session cookie")
	}
}
