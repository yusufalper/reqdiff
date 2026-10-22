package compare

import (
	"encoding/json"
	"math/big"
	"reqdiff/internal/report"
	"sort"
	"strconv"
)

func kind(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case json.Number:
		return "number"
	case string:
		return "string"
	case bool:
		return "boolean"
	default:
		return "null"
	}
}
func value(v any) *any {
	if kind(v) == "object" || kind(v) == "array" {
		return nil
	}
	return &v
}
func JSON(left, right any, mode string, ignore, ignoreValues Paths) []report.Difference {
	out := []report.Difference{}
	var walk func(any, any, bool, bool, []string)
	walk = func(l, r any, lExists, rExists bool, path []string) {
		if ignore.Covers(path) {
			return
		}
		lt, rt := kind(l), kind(r)
		d := report.Difference{Scope: "body", Path: pointer(path), LeftType: lt, RightType: rt}
		if !lExists {
			d.Kind = "added"
			d.LeftType = ""
		} else if !rExists {
			d.Kind = "removed"
			d.RightType = ""
		} else if lt != rt {
			d.Kind = "type_changed"
		}
		if d.Kind != "" {
			if mode == "values" && !ignoreValues.Covers(path) {
				if lExists {
					d.LeftValue = value(l)
				}
				if rExists {
					d.RightValue = value(r)
				}
			}
			out = append(out, d)
			return
		}
		switch lv := l.(type) {
		case map[string]any:
			rv := r.(map[string]any)
			keys := map[string]bool{}
			for k := range lv {
				keys[k] = true
			}
			for k := range rv {
				keys[k] = true
			}
			for k := range keys {
				a, aOK := lv[k]
				b, bOK := rv[k]
				walk(a, b, aOK, bOK, append(path, k))
			}
		case []any:
			rv := r.([]any)
			n := max(len(lv), len(rv))
			for i := 0; i < n; i++ {
				var a, b any
				if i < len(lv) {
					a = lv[i]
				}
				if i < len(rv) {
					b = rv[i]
				}
				walk(a, b, i < len(lv), i < len(rv), append(path, strconv.Itoa(i)))
			}
		default:
			if mode != "values" || ignoreValues.Covers(path) {
				return
			}
			equal := l == r
			if lt == "number" {
				a, _ := new(big.Rat).SetString(string(l.(json.Number)))
				b, _ := new(big.Rat).SetString(string(r.(json.Number)))
				equal = a != nil && b != nil && a.Cmp(b) == 0
			}
			if !equal {
				d.Kind = "value_changed"
				d.LeftValue = value(l)
				d.RightValue = value(r)
				out = append(out, d)
			}
		}
	}
	walk(left, right, true, true, nil)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}
