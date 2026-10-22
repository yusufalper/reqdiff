package compare

import (
	"errors"
	"github.com/yusufalper/reqdiff/internal/config"
	"github.com/yusufalper/reqdiff/internal/httpfetch"
	"github.com/yusufalper/reqdiff/internal/jsondoc"
	"github.com/yusufalper/reqdiff/internal/report"
	"net/url"
	"sort"
)

func endpoint(s httpfetch.Snapshot, rawURL string) report.Endpoint {
	e := report.Endpoint{DurationNS: s.Duration.Nanoseconds(), BodyBytes: len(s.Body)}
	if u, err := url.Parse(rawURL); err == nil {
		u.User = nil
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		e.URL = u.String()
	}
	if s.Status > 0 {
		status := s.Status
		e.Status = &status
	}
	return e
}
func Responses(left, right httpfetch.Snapshot, opts config.Options) (report.Report, error) {
	r := report.New(opts.Mode)
	r.Left, r.Right = endpoint(left, opts.Left.URL), endpoint(right, opts.Right.URL)
	r.Latency = report.Latency{LeftNS: left.Duration.Nanoseconds(), RightNS: right.Duration.Nanoseconds(), DeltaNS: (right.Duration - left.Duration).Nanoseconds()}
	if opts.MaxLatencyRegression != nil {
		n := opts.MaxLatencyRegression.Nanoseconds()
		r.Latency.ThresholdNS = &n
		r.Latency.Exceeded = right.Duration-left.Duration > *opts.MaxLatencyRegression
	}
	var bodies [2]any
	valid := true
	for i, side := range []struct {
		name string
		s    httpfetch.Snapshot
		e    *report.Endpoint
	}{{"left", left, &r.Left}, {"right", right, &r.Right}} {
		if side.s.Status == 0 {
			valid = false
			continue
		}
		if side.s.Status >= 400 {
			r.Warnings = append(r.Warnings, report.Warning{Side: side.name, Code: "http_error_status", Message: "HTTP error status; equality does not imply a healthy endpoint"})
		}
		side.e.BodyKind = "empty"
		if len(side.s.Body) == 0 {
			continue
		}
		v, err := jsondoc.Decode(side.s.Body, 128)
		if err != nil {
			code := "invalid_json"
			if errors.Is(err, jsondoc.ErrLimit) {
				code = "limit_exceeded"
			}
			r.Errors = append(r.Errors, report.Error{Side: side.name, Code: code, Message: err.Error()})
			side.e.BodyKind = "invalid"
			valid = false
			continue
		}
		bodies[i] = v
		side.e.BodyKind = "json"
	}
	if left.Status > 0 && right.Status > 0 {
		if left.Status != right.Status {
			r.Differences = append(r.Differences, report.Difference{Scope: "status", Path: "", Kind: "value_changed", LeftType: "number", RightType: "number", LeftValue: value(left.Status), RightValue: value(right.Status)})
		}
		headers, err := Headers(left.Headers, right.Headers, opts.CompareHeaders)
		if err != nil {
			var e report.Error
			if errors.As(err, &e) {
				r.Errors = append(r.Errors, e)
			}
		} else {
			r.Differences = append(r.Differences, headers...)
		}
		ignore, err := CompilePaths(opts.Ignore)
		if err != nil {
			return r, err
		}
		ignoreValues, err := CompilePaths(opts.IgnoreValues)
		if err != nil {
			return r, err
		}
		if valid && !ignore.Covers(nil) {
			if r.Left.BodyKind != r.Right.BodyKind {
				lt, rt := kind(bodies[0]), kind(bodies[1])
				if r.Left.BodyKind == "empty" {
					lt = "empty"
				}
				if r.Right.BodyKind == "empty" {
					rt = "empty"
				}
				r.Differences = append(r.Differences, report.Difference{Scope: "body", Path: "", Kind: "type_changed", LeftType: lt, RightType: rt})
			} else if r.Left.BodyKind == "json" {
				r.Differences = append(r.Differences, JSON(bodies[0], bodies[1], opts.Mode, ignore, ignoreValues)...)
			}
		}
	}
	sort.SliceStable(r.Differences, func(i, j int) bool {
		a, b := r.Differences[i], r.Differences[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Kind < b.Kind
	})
	if len(r.Differences) > 0 || r.Latency.Exceeded {
		r.Result = "different"
	}
	if len(r.Errors) > 0 {
		r.Result = "error"
		return r, r.Errors[0]
	}
	return r, nil
}
