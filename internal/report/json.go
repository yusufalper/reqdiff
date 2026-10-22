package report

import (
	"encoding/json"
	"io"
)

func sanitized(r Report, redact func(string) string) Report {
	if redact == nil {
		redact = func(s string) string { return s }
	}
	r.Left.URL = redact(r.Left.URL)
	r.Right.URL = redact(r.Right.URL)
	r.Differences = append([]Difference{}, r.Differences...)
	for i := range r.Differences {
		d := &r.Differences[i]
		d.Path = redact(d.Path)
		for _, p := range []**any{&d.LeftValue, &d.RightValue} {
			if *p != nil {
				if s, ok := (**p).(string); ok {
					var v any = redact(s)
					*p = &v
				}
			}
		}
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
