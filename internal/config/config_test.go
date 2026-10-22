package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	o := Defaults()
	if o.Mode != "structure" || o.Timeout != 10*time.Second || o.MaxBodyBytes != 10485760 || len(o.CompareHeaders) != 1 {
		t.Fatalf("%+v", o)
	}
}
func TestConfigStrict(t *testing.T) {
	for _, s := range []string{`{"typo":1}`, `{"timeout":"-1s"}`, `{"timeout":"1s","timeout":"2s"}`, `{} {}`} {
		p := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(p, []byte(s), 0600)
		if _, err := Load(p); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
func TestLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte(`{"left":{"url":"https://a.test"},"right":{"url":"https://b.test"},"timeout":"2s","compare":{"mode":"values","ignore":["/id"]}}`), 0600)
	o, err := Load(p)
	if err != nil || o.Mode != "values" || o.Timeout != 2*time.Second || o.Ignore[0] != "/id" {
		t.Fatalf("%+v %v", o, err)
	}
}
func TestValidation(t *testing.T) {
	for _, u := range []string{"ftp://a.test", "https://u:p@a.test", "https://a.test/#x", "relative"} {
		o := Defaults()
		o.Left.URL = u
		o.Right.URL = "https://b.test"
		if o.Validate() == nil {
			t.Errorf("accepted %s", u)
		}
	}
	o := Defaults()
	o.Left.URL = "https://a.test"
	o.Right.URL = "https://b.test"
	o.Left.Headers = map[string]string{"Host": "elsewhere"}
	if o.Validate() == nil {
		t.Fatal("accepted host override")
	}
}

func TestEndpointAuthAndHeaders(t *testing.T) {
	cases := []Endpoint{
		{Auth: Auth{Type: "bearer"}},
		{Auth: Auth{Type: "basic", UsernameEnv: "USER"}},
		{Auth: Auth{Type: "none", TokenEnv: "TOKEN"}},
		{Auth: Auth{Type: "oauth"}},
		{Auth: Auth{Type: "bearer", TokenEnv: "TOKEN"}, HeaderEnv: map[string]string{"Authorization": "OTHER"}},
		{CookieEnv: "COOKIE", HeaderEnv: map[string]string{"Cookie": "OTHER"}},
		{Headers: map[string]string{"X-Key": "line\r\nInjected: bad"}},
		{Headers: map[string]string{"X-Key": "one", "x-key": "two"}},
		{Headers: map[string]string{"Authorization": "secret"}},
		{Headers: map[string]string{"Cookie": "secret"}},
		{Headers: map[string]string{"Accept-Encoding": "gzip"}},
		{HeaderEnv: map[string]string{"Bad Header": "KEY"}},
	}
	for i, e := range cases {
		e.URL = "https://example.com"
		if err := e.Validate(); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	for _, a := range []Auth{{}, {Type: "none"}, {Type: "bearer", TokenEnv: "T"}, {Type: "basic", UsernameEnv: "U", PasswordEnv: "P"}} {
		e := Endpoint{URL: "https://example.com?a=b", Auth: a}
		if err := e.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestOptionValidation(t *testing.T) {
	for _, mutate := range []func(*Options){
		func(o *Options) { o.Mode = "unknown" }, func(o *Options) { o.Format = "unknown" },
		func(o *Options) { o.Timeout = 0 }, func(o *Options) { o.MaxBodyBytes = 0 },
		func(o *Options) { d := -time.Second; o.MaxLatencyRegression = &d },
		func(o *Options) { o.CompareHeaders = []string{"Bad Header"} },
	} {
		o := Defaults()
		o.Left.URL = "https://a.test"
		o.Right.URL = "https://b.test"
		mutate(&o)
		if o.Validate() == nil {
			t.Fatal("invalid options accepted")
		}
	}
}
func TestConfigSizeAndDurationLimits(t *testing.T) {
	for _, s := range []string{strings.Repeat(" ", 1<<20) + "{}", `{"timeout":"oops"}`, `{"compare":{"max_latency_regression":"-1ms"}}`, `{"compare":{"max_latency_regression":"oops"}}`} {
		p := filepath.Join(t.TempDir(), "config.json")
		os.WriteFile(p, []byte(s), 0600)
		if _, err := Load(p); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
