package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseFlagsAfterURLs(t *testing.T) {
	o, err := Parse([]string{"https://a.test", "https://b.test", "--mode", "values", "--timeout=3s"})
	if err != nil || o.Mode != "values" || o.Timeout != 3*time.Second {
		t.Fatalf("%+v %v", o, err)
	}
}
func TestParseConfigPrecedence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(`{"left":{"url":"https://a.test","auth":{"type":"basic","username_env":"U","password_env":"P"}},"right":{"url":"https://b.test"},"compare":{"mode":"values","ignore":["/old"]},"timeout":"2s"}`), 0600)
	o, err := Parse([]string{"--config", p, "--left-bearer-env", "TOKEN", "--ignore", "/new"})
	if err != nil || o.Left.Auth.Type != "bearer" || o.Left.Auth.UsernameEnv != "" || o.Timeout != 2*time.Second || len(o.Ignore) != 1 || o.Ignore[0] != "/new" {
		t.Fatalf("%+v %v", o, err)
	}
}
func TestParseInvalid(t *testing.T) {
	for _, a := range [][]string{{"https://a.test"}, {"--unknown"}, {"https://a.test", "https://b.test", "--timeout", "-1s"}, {"--help=garbage"}} {
		if _, err := Parse(a); err == nil {
			t.Errorf("accepted %v", a)
		}
	}
}
