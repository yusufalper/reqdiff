package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func noEnv(string) (string, bool) { return "", false }
func server(t *testing.T, body string) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(s.Close)
	return s
}
func TestRunEqualDifferentError(t *testing.T) {
	a, b := server(t, `{"id":1}`), server(t, `{"id":2}`)
	for _, tc := range []struct {
		args   []string
		exit   int
		result string
	}{
		{[]string{a.URL, b.URL, "--format", "json"}, 0, "equal"},
		{[]string{a.URL, b.URL, "--mode", "values", "--format", "json"}, 1, "different"},
		{[]string{a.URL, b.URL, "--mode", "oops", "--format", "json"}, 2, "error"},
		{[]string{a.URL, b.URL, "--unknown", "--format", "json"}, 2, "error"},
		{[]string{"--config", "/does-not-exist", "--format=json"}, 2, "error"},
	} {
		var out, errout bytes.Buffer
		code := Run(context.Background(), tc.args, &out, &errout, noEnv)
		var r map[string]any
		err := json.Unmarshal(out.Bytes(), &r)
		if code != tc.exit || err != nil || r["result"] != tc.result || errout.Len() != 0 {
			t.Fatalf("args %v: exit %d json %v output %s stderr %s", tc.args, code, err, out.String(), errout.String())
		}
	}
}
func TestRunConcurrent(t *testing.T) {
	l, r := make(chan struct{}), make(chan struct{})
	makeServer := func(own chan struct{}, other chan struct{}) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			close(own)
			select {
			case <-other:
				fmt.Fprint(w, "{}")
			case <-req.Context().Done():
			}
		}))
	}
	a, b := makeServer(l, r), makeServer(r, l)
	defer a.Close()
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out, errout bytes.Buffer
	if code := Run(ctx, []string{a.URL, b.URL}, &out, &errout, noEnv); code != 0 {
		t.Fatalf("exit %d: %s", code, errout.String())
	}
}
func TestInvalidInputsMakeNoRequests(t *testing.T) {
	var hits atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); fmt.Fprint(w, "{}") }))
	defer s.Close()
	for _, extra := range [][]string{{"--right-bearer-env", "MISSING"}, {"--ignore", "bad"}} {
		var out, errout bytes.Buffer
		code := Run(context.Background(), append([]string{s.URL, s.URL}, extra...), &out, &errout, noEnv)
		if code != 2 || hits.Load() != 0 {
			t.Fatalf("exit %d hits %d", code, hits.Load())
		}
	}
}
func TestRunPartialFailureAndCancellation(t *testing.T) {
	good := server(t, "{}")
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	var out, errout bytes.Buffer
	code := Run(context.Background(), []string{good.URL, slow.URL, "--timeout", "30ms", "--format=json"}, &out, &errout, noEnv)
	var r struct {
		Left   struct{ Status *int }
		Errors []struct{ Side, Code string }
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || code != 2 || r.Left.Status == nil || len(r.Errors) != 1 || r.Errors[0].Side != "right" || r.Errors[0].Code != "timeout" {
		t.Fatalf("exit %d, %s", code, out.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	if code = Run(ctx, []string{good.URL, good.URL, "--format=json"}, &out, &errout, noEnv); code != 130 {
		t.Fatal(code)
	}
}
func TestRunAuthIsolationAndNoLeaks(t *testing.T) {
	makeServer := func(token string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+token {
				t.Error("wrong token")
			}
			fmt.Fprintf(w, `{"echo":%q}`, token)
		}))
	}
	a, b := makeServer("left-secret"), makeServer("right-secret")
	defer a.Close()
	defer b.Close()
	var out, errout bytes.Buffer
	lookup := func(name string) (string, bool) {
		if name == "LEFT" {
			return "left-secret", true
		}
		return "right-secret", true
	}
	code := Run(context.Background(), []string{a.URL + "?key=hidden-query", b.URL, "--left-bearer-env", "LEFT", "--right-bearer-env", "RIGHT", "--mode=values", "--format=json"}, &out, &errout, lookup)
	if code != 1 || strings.Contains(out.String(), "left-secret") || strings.Contains(out.String(), "right-secret") || strings.Contains(out.String(), "hidden-query") {
		t.Fatalf("exit %d: %s", code, out.String())
	}
}
func TestHelpAndVersion(t *testing.T) {
	for _, arg := range []string{"--help", "--version"} {
		var b bytes.Buffer
		if Run(context.Background(), []string{arg}, &b, &b, noEnv) != 0 || b.Len() == 0 {
			t.Fatal(arg)
		}
	}
}

func TestRunRedactsEncodedCredentials(t *testing.T) {
	for _, secret := range []string{"abc/def~xyz", `a"b\c<d>`} {
		t.Run(secret, func(t *testing.T) {
			left := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Echo", secret)
				json.NewEncoder(w).Encode(map[string]any{secret: true})
			}))
			defer left.Close()
			right := server(t, "{}")
			for _, format := range []string{"json", "text"} {
				var out, errout bytes.Buffer
				args := []string{left.URL + "/" + url.PathEscape(secret), right.URL, "--left-header-env", "X-Key=KEY", "--right-header-env", "X-Key=KEY", "--compare-header", "X-Echo", "--format", format}
				code := Run(context.Background(), args, &out, &errout, func(string) (string, bool) { return secret, true })
				if code != 1 {
					t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
				}
				// Decode each display encoding independently: reversible escaping is not redaction.
				if format == "json" {
					var result struct {
						Left        struct{ URL string }
						Differences []struct {
							Scope, Path string
							LeftValue   *string `json:"left_value"`
						}
					}
					if err := json.Unmarshal(out.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					decodedURL, _ := url.PathUnescape(result.Left.URL)
					if strings.Contains(decodedURL, secret) {
						t.Error("credential in URL path")
					}
					for _, d := range result.Differences {
						path := strings.ReplaceAll(strings.ReplaceAll(d.Path, "~1", "/"), "~0", "~")
						if strings.Contains(path, secret) {
							t.Error("credential in JSON Pointer")
						}
						if d.Scope == "header" && d.LeftValue != nil {
							var values []string
							if err := json.Unmarshal([]byte(*d.LeftValue), &values); err != nil {
								t.Fatal(err)
							}
							for _, v := range values {
								if strings.Contains(v, secret) {
									t.Error("credential in encoded header")
								}
							}
						}
					}
				} else {
					if !strings.Contains(out.String(), "[REDACTED]") {
						t.Error("text report missing redaction")
					}
					pointerEncoded := strings.ReplaceAll(strings.ReplaceAll(secret, "~", "~0"), "/", "~1")
					headerEncoded, _ := json.Marshal([]string{secret})
					headerDisplay, _ := json.Marshal(string(headerEncoded))
					for _, encoded := range []string{pointerEncoded, url.PathEscape(secret), string(headerDisplay)} {
						if strings.Contains(out.String(), encoded) {
							t.Errorf("recoverable credential in text: %s", encoded)
						}
					}
				}
			}
		})
	}
}
