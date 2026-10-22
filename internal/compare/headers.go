package compare

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"reflect"
	"reqdiff/internal/report"
	"sort"
	"strings"
)

func headerValues(h http.Header, name string) ([]string, bool) {
	for k, v := range h {
		if strings.EqualFold(k, name) {
			out := make([]string, len(v))
			for i, s := range v {
				out[i] = strings.TrimSpace(s)
			}
			return out, true
		}
	}
	return nil, false
}
func normalizeHeader(name string, values []string) ([]string, error) {
	if !strings.EqualFold(name, "Content-Type") {
		return values, nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		media, params, err := mime.ParseMediaType(v)
		if err != nil {
			return nil, errors.New("invalid Content-Type header")
		}
		out[i] = mime.FormatMediaType(strings.ToLower(media), params)
	}
	return out, nil
}
func Headers(left, right http.Header, names []string) ([]report.Difference, error) {
	out := []report.Difference{}
	seen := map[string]bool{}
	for _, name := range names {
		name = http.CanonicalHeaderKey(name)
		if seen[name] {
			continue
		}
		seen[name] = true
		l, lOK := headerValues(left, name)
		r, rOK := headerValues(right, name)
		var err error
		l, err = normalizeHeader(name, l)
		if err != nil {
			return nil, report.Error{Side: "left", Code: "invalid_json", Message: err.Error()}
		}
		r, err = normalizeHeader(name, r)
		if err != nil {
			return nil, report.Error{Side: "right", Code: "invalid_json", Message: err.Error()}
		}
		if lOK == rOK && reflect.DeepEqual(l, r) {
			continue
		}
		d := report.Difference{Scope: "header", Path: name, Kind: "value_changed", LeftType: "string", RightType: "string"}
		if !lOK {
			d.Kind = "added"
			d.LeftType = ""
		} else if !rOK {
			d.Kind = "removed"
			d.RightType = ""
		}
		sensitive := name == "Set-Cookie" || name == "Cookie" || name == "Authorization" || name == "Proxy-Authorization"
		encode := func(v []string) *any {
			var text string
			if sensitive {
				text = "[REDACTED]"
			} else {
				data, _ := json.Marshal(v)
				text = string(data)
			}
			return value(text)
		}
		if lOK {
			d.LeftValue = encode(l)
		}
		if rOK {
			d.RightValue = encode(r)
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
