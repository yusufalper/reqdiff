package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

func text(s string) string { q := strconv.Quote(s); return q[1 : len(q)-1] }
func status(s *int) string {
	if s == nil {
		return "—"
	}
	return strconv.Itoa(*s)
}
func display(v *any, typ string) string {
	if v == nil {
		if typ == "" {
			return "missing"
		}
		return typ
	}
	b, _ := json.Marshal(*v)
	return string(b)
}
func WriteText(w io.Writer, r Report, redact func(string) string) error {
	r = sanitized(r, redact)
	var b bytes.Buffer
	if r.Left.URL != "" {
		fmt.Fprintf(&b, "Left:  %s\n", text(r.Left.URL))
	}
	if r.Right.URL != "" {
		fmt.Fprintf(&b, "Right: %s\n", text(r.Right.URL))
	}
	fmt.Fprintf(&b, "Status: %s → %s\n", status(r.Left.Status), status(r.Right.Status))
	fmt.Fprintf(&b, "Latency: %.0fms → %.0fms (%+.0fms)\n", float64(r.Latency.LeftNS)/1e6, float64(r.Latency.RightNS)/1e6, float64(r.Latency.DeltaNS)/1e6)
	if r.Latency.Exceeded {
		fmt.Fprintln(&b, "Latency regression exceeds threshold")
	}
	for _, group := range []struct{ scope, title string }{{"body", "JSON differences"}, {"header", "Header differences"}} {
		first := true
		for _, d := range r.Differences {
			if d.Scope != group.scope {
				continue
			}
			if first {
				fmt.Fprintf(&b, "\n%s:\n", group.title)
				first = false
			}
			path := d.Path
			if path == "" {
				path = "<root>"
			}
			path = text(path)
			switch d.Kind {
			case "added":
				fmt.Fprintf(&b, "+ %s: %s\n", path, display(d.RightValue, d.RightType))
			case "removed":
				fmt.Fprintf(&b, "- %s: %s\n", path, display(d.LeftValue, d.LeftType))
			default:
				fmt.Fprintf(&b, "~ %s: %s → %s\n", path, display(d.LeftValue, d.LeftType), display(d.RightValue, d.RightType))
			}
		}
	}
	for _, warning := range r.Warnings {
		fmt.Fprintf(&b, "\nWarning (%s): %s\n", text(warning.Side), text(warning.Message))
	}
	for _, err := range r.Errors {
		fmt.Fprintf(&b, "\nError (%s/%s): %s\n", text(err.Side), text(err.Code), text(err.Message))
	}
	fmt.Fprintf(&b, "\nResult: %s\n", text(r.Result))
	_, err := w.Write(b.Bytes())
	return err
}
