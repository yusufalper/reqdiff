package auth

import (
	"net/http"
	"reqdiff/internal/config"
	"strings"
	"testing"
)

func TestResolveAuthentication(t *testing.T) {
	env := map[string]string{"L": "left-secret", "R": "right-secret", "U": "me@example.com", "P": "password", "KEY": "key-secret", "C": "sid=cookie-secret"}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	for _, key := range []string{"L", "R"} {
		r, err := Resolve(config.Endpoint{URL: "https://example.com", Auth: config.Auth{Type: "bearer", TokenEnv: key}}, lookup)
		if err != nil || r.Headers.Get("Authorization") != "Bearer "+env[key] {
			t.Fatalf("wrong auth: %v", err)
		}
	}
	r, err := Resolve(config.Endpoint{URL: "https://example.com", Auth: config.Auth{Type: "basic", UsernameEnv: "U", PasswordEnv: "P"}, HeaderEnv: map[string]string{"X-API-Key": "KEY"}, CookieEnv: "C"}, lookup)
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{Header: r.Headers}
	u, p, ok := req.BasicAuth()
	if !ok || u != "me@example.com" || p != "password" || r.Headers.Get("Cookie") != "sid=cookie-secret" || r.Headers.Get("X-API-Key") != "key-secret" {
		t.Fatal("credentials not resolved")
	}
}
func TestResolveRejectsInvalidSecrets(t *testing.T) {
	for _, value := range []string{"", "bad\r\nX-Evil: yes"} {
		_, err := Resolve(config.Endpoint{Auth: config.Auth{Type: "bearer", TokenEnv: "TOKEN"}}, func(string) (string, bool) { return value, true })
		if err == nil || strings.Contains(err.Error(), "X-Evil") {
			t.Fatal("unsafe credential accepted or leaked")
		}
	}
	_, err := Resolve(config.Endpoint{Auth: config.Auth{Type: "basic", UsernameEnv: "U", PasswordEnv: "P"}}, func(string) (string, bool) { return "bad:user", true })
	if err == nil {
		t.Fatal("colon in basic username accepted")
	}
}
func TestRedactorLongestFirst(t *testing.T) {
	r := NewRedactor([]string{"secret", "secret-long", "Bearer secret-long"})
	got := r.String("Bearer secret-long / secret")
	if got != "[REDACTED] / [REDACTED]" {
		t.Fatal(got)
	}
}
