package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reqdiff/internal/jsondoc"
	"strings"
	"time"
)

type Auth struct {
	Type        string `json:"type"`
	TokenEnv    string `json:"token_env"`
	UsernameEnv string `json:"username_env"`
	PasswordEnv string `json:"password_env"`
}
type Endpoint struct {
	URL       string            `json:"url"`
	Auth      Auth              `json:"auth"`
	Headers   map[string]string `json:"headers"`
	HeaderEnv map[string]string `json:"header_env"`
	CookieEnv string            `json:"cookie_env"`
}
type Options struct {
	Left, Right                          Endpoint
	Mode, Format                         string
	Timeout                              time.Duration
	MaxBodyBytes                         int64
	Ignore, IgnoreValues, CompareHeaders []string
	MaxLatencyRegression                 *time.Duration
}

func Defaults() Options {
	return Options{Mode: "structure", Format: "text", Timeout: 10 * time.Second, MaxBodyBytes: 10 << 20, CompareHeaders: []string{"Content-Type"}}
}

// File uses pointers for optional scalars so explicit zero values aren't mistaken for defaults.
type fileConfig struct {
	Left    Endpoint `json:"left"`
	Right   Endpoint `json:"right"`
	Compare struct {
		Mode                 *string  `json:"mode"`
		Ignore               []string `json:"ignore"`
		IgnoreValues         []string `json:"ignore_values"`
		Headers              []string `json:"headers"`
		MaxLatencyRegression *string  `json:"max_latency_regression"`
	} `json:"compare"`
	Timeout      *string `json:"timeout"`
	MaxBodyBytes *int64  `json:"max_body_bytes"`
	Format       *string `json:"format"`
}

func Load(path string) (Options, error) {
	o := Defaults()
	f, err := os.Open(path)
	if err != nil {
		return o, errors.New("cannot open config file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return o, errors.New("cannot read config file")
	}
	if len(data) > 1<<20 {
		return o, errors.New("config exceeds 1 MiB")
	}
	if _, err = jsondoc.Decode(data, 128); err != nil {
		return o, fmt.Errorf("config: %w", err)
	}
	var c fileConfig
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return o, errors.New("invalid config: unknown field or wrong field type")
	}
	o.Left, o.Right = c.Left, c.Right
	if c.Compare.Mode != nil {
		o.Mode = *c.Compare.Mode
	}
	if c.Format != nil {
		o.Format = *c.Format
	}
	o.Ignore, o.IgnoreValues = c.Compare.Ignore, c.Compare.IgnoreValues
	if c.Compare.Headers != nil {
		o.CompareHeaders = c.Compare.Headers
	}
	if c.MaxBodyBytes != nil {
		o.MaxBodyBytes = *c.MaxBodyBytes
	}
	if c.Timeout != nil {
		o.Timeout, err = time.ParseDuration(*c.Timeout)
		if err != nil || o.Timeout <= 0 {
			return o, errors.New("timeout must be a positive duration")
		}
	}
	if c.Compare.MaxLatencyRegression != nil {
		v, err := time.ParseDuration(*c.Compare.MaxLatencyRegression)
		if err != nil || v < 0 {
			return o, errors.New("latency threshold must be non-negative")
		}
		o.MaxLatencyRegression = &v
	}
	return o, nil
}
func (o Options) Validate() error {
	if o.Mode != "structure" && o.Mode != "values" {
		return errors.New("mode must be structure or values")
	}
	if o.Format != "text" && o.Format != "json" {
		return errors.New("format must be text or json")
	}
	if o.Timeout <= 0 || o.MaxBodyBytes <= 0 || o.MaxBodyBytes == int64(^uint64(0)>>1) {
		return errors.New("timeout and body limit must be positive and within range")
	}
	if o.MaxLatencyRegression != nil && *o.MaxLatencyRegression < 0 {
		return errors.New("latency threshold must be non-negative")
	}
	for _, name := range o.CompareHeaders {
		if !ValidHeaderName(name) {
			return errors.New("invalid response header name")
		}
	}
	for _, side := range []struct {
		name string
		e    Endpoint
	}{{"left", o.Left}, {"right", o.Right}} {
		if err := side.e.Validate(); err != nil {
			return fmt.Errorf("%s: %w", side.name, err)
		}
	}
	return nil
}
func (e Endpoint) Validate() error {
	u, err := url.Parse(e.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || strings.Contains(e.URL, "#") {
		return errors.New("URL must be absolute HTTP(S), without userinfo or fragment")
	}
	seen := map[string]bool{}
	for _, m := range []map[string]string{e.Headers, e.HeaderEnv} {
		for k, v := range m {
			name := http.CanonicalHeaderKey(k)
			if !ValidHeaderName(k) || ForbiddenHeader(name) || seen[name] {
				return errors.New("invalid, duplicate, or reserved request header")
			}
			if !ValidHeaderValue(v) {
				return errors.New("invalid request header value")
			}
			seen[name] = true
		}
	}
	for k := range e.Headers {
		if strings.EqualFold(k, "Authorization") || strings.EqualFold(k, "Cookie") {
			return errors.New("use auth, cookie-env, or header-env for credentials")
		}
	}
	a := e.Auth
	switch a.Type {
	case "", "none":
		if a.TokenEnv != "" || a.UsernameEnv != "" || a.PasswordEnv != "" {
			return errors.New("auth fields require an auth type")
		}
	case "bearer":
		if a.TokenEnv == "" || a.UsernameEnv != "" || a.PasswordEnv != "" || seen["Authorization"] {
			return errors.New("bearer auth requires only token_env and no Authorization header")
		}
	case "basic":
		if a.UsernameEnv == "" || a.PasswordEnv == "" || a.TokenEnv != "" || seen["Authorization"] {
			return errors.New("basic auth requires username_env and password_env only")
		}
	default:
		return errors.New("unknown auth type")
	}
	if e.CookieEnv != "" && seen["Cookie"] {
		return errors.New("cookie has multiple sources")
	}
	return nil
}
func ValidHeaderName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return false
		}
	}
	return true
}
func ValidHeaderValue(s string) bool {
	for _, c := range s {
		if c == 127 || c < 32 && c != '\t' {
			return false
		}
	}
	return true
}
func ForbiddenHeader(s string) bool {
	switch http.CanonicalHeaderKey(s) {
	case "Host", "Content-Length", "Transfer-Encoding", "Connection", "Proxy-Authorization", "Accept-Encoding":
		return true
	}
	return false
}
