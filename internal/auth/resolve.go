// Package auth resolves credentials once, before either request is sent.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"reqdiff/internal/config"
	"sort"
	"strings"
)

type ResolvedEndpoint struct {
	URL     string
	Headers http.Header
	Secrets []string
}

type Redactor struct{ secrets []string }

func Resolve(e config.Endpoint, lookup func(string) (string, bool)) (ResolvedEndpoint, error) {
	out := ResolvedEndpoint{URL: e.URL, Headers: make(http.Header)}
	read := func(name string) (string, error) {
		value, ok := lookup(name)
		if !ok || value == "" {
			return "", fmt.Errorf("required environment variable %q is empty or missing", name)
		}
		// Keep resolved secrets even on failure, so the caller can mask partial errors.
		out.Secrets = append(out.Secrets, value)
		if !config.ValidHeaderValue(value) {
			return "", errors.New("credential contains invalid header characters")
		}
		return value, nil
	}
	out.Headers.Set("Accept", "application/json")
	for name, value := range e.Headers {
		out.Headers.Set(name, value)
	}
	for name, env := range e.HeaderEnv {
		value, err := read(env)
		if err != nil {
			return out, err
		}
		out.Headers.Set(name, value)
	}
	switch e.Auth.Type {
	case "bearer":
		token, err := read(e.Auth.TokenEnv)
		if err != nil {
			return out, err
		}
		out.Headers.Set("Authorization", "Bearer "+token)
	case "basic":
		user, err := read(e.Auth.UsernameEnv)
		if err != nil {
			return out, err
		}
		pass, err := read(e.Auth.PasswordEnv)
		if err != nil {
			return out, err
		}
		if strings.Contains(user, ":") {
			return out, errors.New("basic username cannot contain a colon")
		}
		req := &http.Request{Header: out.Headers}
		req.SetBasicAuth(user, pass)
	}
	if e.CookieEnv != "" {
		value, err := read(e.CookieEnv)
		if err != nil {
			return out, err
		}
		out.Headers.Set("Cookie", value)
	}
	if value := out.Headers.Get("Authorization"); value != "" {
		out.Secrets = append(out.Secrets, value)
	}
	return out, nil
}
func NewRedactor(secrets []string) Redactor {
	unique := map[string]bool{}
	list := []string{}
	for _, s := range secrets {
		if s != "" && !unique[s] {
			unique[s] = true
			list = append(list, s)
		}
	}
	sort.Slice(list, func(i, j int) bool { return len(list[i]) > len(list[j]) })
	return Redactor{list}
}
func (r Redactor) String(s string) string {
	// One pass avoids treating the replacement marker as another secret.
	if len(r.secrets) == 0 {
		return s
	}
	pairs := make([]string, 0, len(r.secrets)*2)
	for _, secret := range r.secrets {
		pairs = append(pairs, secret, "[REDACTED]")
	}
	return strings.NewReplacer(pairs...).Replace(s)
}
