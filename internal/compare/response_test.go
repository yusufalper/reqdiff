package compare

import (
	"net/http"
	"reqdiff/internal/config"
	"reqdiff/internal/httpfetch"
	"testing"
	"time"
)

func snap(status int, body string) httpfetch.Snapshot {
	return httpfetch.Snapshot{Status: status, Body: []byte(body), Headers: make(http.Header), Duration: 100 * time.Millisecond}
}
func TestResponseStatusAndEmpty(t *testing.T) {
	o := config.Defaults()
	r, err := Responses(snap(401, `{"error":"no"}`), snap(401, `{"error":"no"}`), o)
	if err != nil || r.Result != "equal" || len(r.Warnings) != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = Responses(snap(204, ""), snap(200, "null"), o)
	if err != nil || r.Result != "different" || len(r.Differences) != 2 {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = Responses(snap(204, ""), snap(204, ""), o)
	if err != nil || r.Result != "equal" {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestHeaders(t *testing.T) {
	l := http.Header{"content-type": []string{"Application/JSON; charset=utf-8; profile=x"}, "Date": []string{"old"}}
	r := http.Header{"Content-Type": []string{"application/json; profile=x; charset=utf-8"}, "Date": []string{"new"}}
	d, err := Headers(l, r, []string{"Content-Type"})
	if err != nil || len(d) != 0 {
		t.Fatalf("%+v %v", d, err)
	}
	d, err = Headers(http.Header{}, http.Header{"X-Test": []string{""}}, []string{"X-Test"})
	if err != nil || len(d) != 1 || d[0].Kind != "added" {
		t.Fatalf("%+v %v", d, err)
	}
	d, err = Headers(http.Header{"X-Test": []string{"a", "b"}}, http.Header{"X-Test": []string{"b", "a"}}, []string{"X-Test"})
	if err != nil || len(d) != 1 {
		t.Fatal(d, err)
	}
	if _, err = Headers(http.Header{"Content-Type": []string{"bad; ="}}, http.Header{}, []string{"Content-Type"}); err == nil {
		t.Fatal("invalid Content-Type accepted")
	}
}
func TestLatencyThreshold(t *testing.T) {
	for _, tc := range []struct {
		right     time.Duration
		threshold *time.Duration
		exceeded  bool
	}{{250 * time.Millisecond, duration(150 * time.Millisecond), false}, {251 * time.Millisecond, duration(150 * time.Millisecond), true}, {2 * time.Second, nil, false}, {20 * time.Millisecond, duration(0), false}} {
		o := config.Defaults()
		o.MaxLatencyRegression = tc.threshold
		l, r := snap(200, "{}"), snap(200, "{}")
		r.Duration = tc.right
		report, err := Responses(l, r, o)
		if err != nil || report.Latency.Exceeded != tc.exceeded {
			t.Fatalf("%+v %v", report, err)
		}
	}
}
func duration(d time.Duration) *time.Duration { return &d }
func TestInvalidResponseJSON(t *testing.T) {
	r, err := Responses(snap(200, "{}"), snap(200, "<html>"), config.Defaults())
	if err == nil || len(r.Errors) != 1 || r.Errors[0].Side != "right" || r.Errors[0].Code != "invalid_json" || r.Left.Status == nil {
		t.Fatalf("%+v %v", r, err)
	}
}
