package report

import (
	"encoding/json"
	"io"
	"net/url"
	"strings"
)

func sanitized(r Report, redact func(string) string) Report {
	if redact == nil {
		redact = func(s string) string { return s }
	}
	r.Left.URL = redactURL(r.Left.URL, redact)
	r.Right.URL = redactURL(r.Right.URL, redact)
	r.Differences = append([]Difference{}, r.Differences...)
	for i := range r.Differences {
		d := &r.Differences[i]
		if d.Scope == "body" {
			d.Path = redactPointer(d.Path, redact)
		} else {
			d.Path = redact(d.Path)
		}
		d.LeftValue = redactValue(d.LeftValue, d.Scope, redact)
		d.RightValue = redactValue(d.RightValue, d.Scope, redact)
	}
	r.Errors = append([]Error{}, r.Errors...)
	for i := range r.Errors {
		r.Errors[i].Message = redact(r.Errors[i].Message)
	}
	r.Warnings = append([]Warning{}, r.Warnings...)
	for i := range r.Warnings {
		r.Warnings[i].Message = redact(r.Warnings[i].Message)
	}
	return r
}
func WriteJSON(w io.Writer, r Report, redact func(string) string) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(sanitized(r, redact))
}

// Paths and header lists are already display-encoded by the comparator. Match
// credentials against their original strings, then restore the display encoding.
func redactPointer(path string, redact func(string) string) string {
	if path == "" {
		return path
	}
	parts := strings.Split(path[1:], "/")
	for i, part := range parts {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		part = redact(part)
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return "/" + strings.Join(parts, "/")
}
func redactHeader(s string, redact func(string) string) string {
	var values []string
	if json.Unmarshal([]byte(s), &values) != nil {
		return redact(s)
	}
	for i := range values {
		values[i] = redact(values[i])
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}
func redactURL(s string, redact func(string) string) string {
	u, err := url.Parse(s)
	if err != nil {
		return redact(s)
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	if path := redact(u.Path); path != u.Path {
		u.Path, u.RawPath = path, ""
	}
	return redact(u.String())
}

func redactValue(v *any, scope string, redact func(string) string) *any {
	if v == nil {
		return nil
	}
	s, ok := (*v).(string)
	if !ok {
		return v
	}
	if scope == "header" {
		s = redactHeader(s, redact)
	} else {
		s = redact(s)
	}
	var cleaned any = s
	return &cleaned
}
