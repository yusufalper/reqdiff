package httpfetch

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/yusufalper/reqdiff/internal/auth"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchSnapshot(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("X-Key") != "abc" {
			t.Error("wrong request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	}))
	defer s.Close()
	c := NewClient(time.Second)
	defer c.CloseIdleConnections()
	v, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL, Headers: http.Header{"X-Key": []string{"abc"}}}, 1024)
	if err != nil || v.Status != 401 || string(v.Body) != `{"error":"unauthorized"}` || v.Duration <= 0 {
		t.Fatalf("%+v %v", v, err)
	}
}
func TestFetchBodyLimit(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if compressed {
				w.Header().Set("Content-Encoding", "gzip")
				z := gzip.NewWriter(w)
				fmt.Fprint(z, "12345")
				z.Close()
			} else {
				fmt.Fprint(w, "12345")
			}
		}))
		c := NewClient(time.Second)
		if _, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL}, 5); err != nil {
			t.Fatal(err)
		}
		if _, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL}, 4); err == nil {
			t.Fatal("accepted oversized body")
		}
		c.CloseIdleConnections()
		s.Close()
	}
}
func TestRedirectNotFollowed(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	c := NewClient(time.Second)
	defer c.CloseIdleConnections()
	v, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: source.URL}, 1024)
	if err != nil || v.Status != 302 || hits.Load() != 0 {
		t.Fatalf("status %d hits %d error %v", v.Status, hits.Load(), err)
	}
}
func TestTimeoutAndCancellation(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	c := NewClient(20 * time.Millisecond)
	defer c.CloseIdleConnections()
	if _, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL}, 1024); err == nil {
		t.Fatal("timeout missing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Fetch(ctx, c, auth.ResolvedEndpoint{URL: s.URL}, 1024); err == nil {
		t.Fatal("cancellation missing")
	}
}
func TestTLSVerification(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer s.Close()
	c := NewClient(time.Second)
	defer c.CloseIdleConnections()
	_, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL}, 1024)
	if err == nil || strings.Contains(err.Error(), s.URL) {
		t.Fatal("expected safe TLS error")
	}
}

func TestFetchMeasuresBodyRead(t *testing.T) {
	sent, release := make(chan struct{}), make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		close(sent)
		<-release
		fmt.Fprint(w, "{}")
	}))
	defer s.Close()
	c := NewClient(time.Second)
	defer c.CloseIdleConnections()
	type result struct {
		s   Snapshot
		err error
	}
	done := make(chan result, 1)
	go func() {
		s, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL}, 1024)
		done <- result{s, err}
	}()
	<-sent
	// Fetch must still be blocked after headers; the body is part of the measured request.
	select {
	case <-done:
		t.Fatal("returned before reading body")
	default:
	}
	close(release)
	r := <-done
	if r.err != nil || string(r.s.Body) != "{}" || r.s.Duration <= 0 {
		t.Fatalf("%+v %v", r.s, r.err)
	}
}
func TestTimeoutDuringBodyRead(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer s.Close()
	c := NewClient(20 * time.Millisecond)
	defer c.CloseIdleConnections()
	snapshot, err := Fetch(context.Background(), c, auth.ResolvedEndpoint{URL: s.URL}, 1024)
	var e *Error
	if !errors.As(err, &e) || e.Code != "timeout" || snapshot.Status != 200 {
		t.Fatalf("%+v %v", snapshot, err)
	}
}
