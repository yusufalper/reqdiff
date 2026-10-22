package config

import (
	"os"
	"path/filepath"
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
