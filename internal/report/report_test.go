package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func sample() Report {
	r := New("structure")
	r.Result = "different"
	a, b := 200, 200
	r.Left.Status = &a
	r.Right.Status = &b
	r.Latency = Latency{LeftNS: 84000000, RightNS: 210000000, DeltaNS: 126000000}
	r.Differences = []Difference{{Scope: "body", Path: "/user/avatar", Kind: "added", RightType: "string"}, {Scope: "body", Path: "/user/id", Kind: "type_changed", LeftType: "number", RightType: "string"}}
	return r
}
func TestTextGolden(t *testing.T) {
	var b bytes.Buffer
	if err := WriteText(&b, sample(), func(s string) string { return s }); err != nil {
		t.Fatal(err)
	}
	want := "Status: 200 → 200\nLatency: 84ms → 210ms (+126ms)\n\nJSON differences:\n+ /user/avatar: string\n~ /user/id: number → string\n\nResult: different\n"
	if b.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}
func TestJSONSchemaV1(t *testing.T) {
	r := sample()
	var null any
	r.Differences = append(r.Differences, Difference{Scope: "body", Path: "/null", Kind: "added", RightType: "null", RightValue: &null})
	var b bytes.Buffer
	if err := WriteJSON(&b, r, func(s string) string { return s }); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != float64(1) || got["result"] != "different" {
		t.Fatal(got)
	}
	ds := got["differences"].([]any)
	last := ds[2].(map[string]any)
	if _, exists := last["left_value"]; exists {
		t.Fatal("missing side serialized")
	}
	if v, ok := last["right_value"]; !ok || v != nil {
		t.Fatal(last)
	}
}
func TestReportRedactionAndControlEscaping(t *testing.T) {
	r := sample()
	r.Left.URL = "https://example.com/\x1b[31m"
	r.Errors = append(r.Errors, Error{Side: "right", Code: "network", Message: "token-secret failed"})
	var secret any = "token-secret"
	r.Differences[0].Path = "/token-secret\n"
	r.Differences[0].RightValue = &secret
	redact := func(s string) string { return strings.ReplaceAll(s, "token-secret", "[REDACTED]") }
	for _, writer := range []func(*bytes.Buffer) error{
		func(b *bytes.Buffer) error { return WriteText(b, r, redact) },
		func(b *bytes.Buffer) error { return WriteJSON(b, r, redact) },
	} {
		var b bytes.Buffer
		if err := writer(&b); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(b.String(), "token-secret") || strings.Contains(b.String(), "\x1b") {
			t.Fatal("unsafe report", b.String())
		}
	}
	if r.Differences[0].Path != "/token-secret\n" {
		t.Fatal("mutated original report")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }
func TestWriteFailure(t *testing.T) {
	if WriteText(brokenWriter{}, sample(), func(s string) string { return s }) == nil {
		t.Fatal("lost text error")
	}
	if WriteJSON(brokenWriter{}, sample(), func(s string) string { return s }) == nil {
		t.Fatal("lost JSON error")
	}
}
