package jsondoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeStrict(t *testing.T) {
	for _, input := range []string{`{"a":1,"a":2}`, `{"a":{"x":0,"x":1}}`, `{} []`, `{`, ""} {
		if _, err := Decode([]byte(input), 128); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	v, err := Decode([]byte(`{"n":9007199254740993} `), 128)
	if err != nil || v.(map[string]any)["n"].(json.Number).String() != "9007199254740993" {
		t.Fatalf("value=%v err=%v", v, err)
	}
}
func TestDecodeLimits(t *testing.T) {
	for _, tc := range []struct {
		s  string
		ok bool
	}{
		{strings.Repeat("[", 128) + "0" + strings.Repeat("]", 128), true},
		{strings.Repeat("[", 129) + "0" + strings.Repeat("]", 129), false},
		{strings.Repeat("1", 1024), true}, {strings.Repeat("1", 1025), false},
		{"1e10000", true}, {"1e10001", false}, {"1e-10001", false},
	} {
		_, err := Decode([]byte(tc.s), 128)
		if (err == nil) != tc.ok {
			t.Errorf("length %d: %v", len(tc.s), err)
		}
	}
}
