package app

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSessionRoundTrip(t *testing.T) {
	pw := "hunter2"
	exp := time.Now().Add(time.Hour).Unix()
	if !validSession(signSession(exp, pw), pw) {
		t.Fatal("a freshly signed session should validate")
	}
}

func TestSessionRejects(t *testing.T) {
	pw := "hunter2"
	future := time.Now().Add(time.Hour).Unix()
	valid := signSession(future, pw)

	cases := map[string]string{
		"empty":           "",
		"no separator":    "12345",
		"garbage mac":     strconv.FormatInt(future, 10) + ".deadbeef",
		"expired":         signSession(time.Now().Add(-time.Minute).Unix(), pw),
		"non-numeric exp": "later." + strings.SplitN(valid, ".", 2)[1],
		"extended expiry": strconv.FormatInt(future+99999, 10) + "." + strings.SplitN(valid, ".", 2)[1],
	}
	for name, value := range cases {
		if validSession(value, pw) {
			t.Errorf("%s: should not validate", name)
		}
	}

	// Changing the password must invalidate sessions signed with the old one.
	if validSession(valid, "different") {
		t.Error("session signed with another password should not validate")
	}
}

func TestSafeNext(t *testing.T) {
	tests := map[string]string{
		"/accounts":            "/accounts",
		"/?month=2026-08":      "/?month=2026-08",
		"":                     "/",
		"https://evil.example": "/",
		"//evil.example":       "/",
		"evil.example":         "/",
		"javascript:alert(1)":  "/",
		// Browsers turn these into "//evil.example" — off-site — even though
		// they start with a single slash.
		"/\\evil.example":   "/",
		"/\\/evil.example":  "/",
		"/\t/evil.example":  "/",
		"/\n//evil.example": "/",
		"/ok/path":          "/ok/path",
	}
	for in, want := range tests {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
