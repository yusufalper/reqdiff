package cli

import (
	"errors"
	"flag"
	"github.com/yusufalper/reqdiff/internal/config"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrHelp = errors.New("help requested")
var ErrVersion = errors.New("version requested")

const Usage = `reqdiff — compare live API responses across environments

Usage:
  reqdiff [options] <left-url> <right-url>
  reqdiff --config reqdiff.json [options]

Comparison:
  --mode structure|values          Default: structure
  --ignore POINTER                 Ignore a field and its subtree (repeatable)
  --ignore-value POINTER           Ignore values, keep presence and type checks
  --compare-header NAME            Response header to compare (default: Content-Type)
  --max-latency-regression 150ms    Fail if the right side is slower by more
  --format text|json               Default: text
  --timeout 10s                    Total timeout per request
  --max-body-bytes 10485760         Maximum decompressed response size
  --config FILE                    Read explicit JSON configuration

Authentication (replace left with right for the other environment):
  --left-bearer-env NAME            Environment variable holding a Bearer token
  --left-basic-user-env NAME        Basic Auth username environment variable
  --left-basic-password-env NAME    Basic Auth password environment variable
  --left-header 'Name: value'       Non-secret request header (repeatable)
  --left-header-env 'Name=ENV'      Secret request header (repeatable)
  --left-cookie-env NAME            Environment variable holding a Cookie header

Exit codes: 0 equal, 1 different, 2 error, 130 cancelled.
Flags may appear before or after the URLs. Redirects are not followed.
`

type listFlag []string

func (l *listFlag) String() string     { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error { *l = append(*l, v); return nil }

type endpointFlags struct {
	bearer, user, password, cookie string
	headers, envHeaders            listFlag
}

func Parse(args []string) (config.Options, error) {
	o := config.Defaults()
	fs := flag.NewFlagSet("reqdiff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var file string
	var help, version bool
	var ignore, values, headers listFlag
	var latency time.Duration
	var left, right endpointFlags
	fs.StringVar(&file, "config", "", "")
	fs.StringVar(&o.Mode, "mode", o.Mode, "")
	fs.StringVar(&o.Format, "format", o.Format, "")
	fs.DurationVar(&o.Timeout, "timeout", o.Timeout, "")
	fs.Int64Var(&o.MaxBodyBytes, "max-body-bytes", o.MaxBodyBytes, "")
	fs.DurationVar(&latency, "max-latency-regression", 0, "")
	fs.Var(&ignore, "ignore", "")
	fs.Var(&values, "ignore-value", "")
	fs.Var(&headers, "compare-header", "")
	fs.BoolVar(&help, "help", false, "")
	fs.BoolVar(&help, "h", false, "")
	fs.BoolVar(&version, "version", false, "")
	for _, side := range []struct {
		name string
		f    *endpointFlags
	}{{"left", &left}, {"right", &right}} {
		p, f := side.name+"-", side.f
		fs.StringVar(&f.bearer, p+"bearer-env", "", "")
		fs.StringVar(&f.user, p+"basic-user-env", "", "")
		fs.StringVar(&f.password, p+"basic-password-env", "", "")
		fs.StringVar(&f.cookie, p+"cookie-env", "", "")
		fs.Var(&f.headers, p+"header", "")
		fs.Var(&f.envHeaders, p+"header-env", "")
	}
	flags, pos, err := reorder(args, fs)
	if err != nil {
		return o, err
	}
	if err = fs.Parse(flags); err != nil {
		return o, errors.New("invalid option value; see --help")
	}
	if help {
		return o, ErrHelp
	}
	if version {
		return o, ErrVersion
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	cli := o
	if set["config"] {
		o, err = config.Load(file)
		if err != nil {
			return o, err
		}
	}
	if set["mode"] {
		o.Mode = cli.Mode
	}
	if set["format"] {
		o.Format = cli.Format
	}
	if set["timeout"] {
		o.Timeout = cli.Timeout
	}
	if set["max-body-bytes"] {
		o.MaxBodyBytes = cli.MaxBodyBytes
	}
	if set["max-latency-regression"] {
		o.MaxLatencyRegression = &latency
	}
	if set["ignore"] {
		o.Ignore = ignore
	}
	if set["ignore-value"] {
		o.IgnoreValues = values
	}
	if set["compare-header"] {
		o.CompareHeaders = headers
	}
	if len(pos) == 2 {
		o.Left.URL, o.Right.URL = pos[0], pos[1]
	} else if len(pos) != 0 || !set["config"] {
		return o, errors.New("provide two URLs or --config FILE")
	}
	for _, side := range []struct {
		name string
		e    *config.Endpoint
		f    endpointFlags
	}{{"left", &o.Left, left}, {"right", &o.Right, right}} {
		p, e, f := side.name+"-", side.e, side.f
		bearer := set[p+"bearer-env"]
		basic := set[p+"basic-user-env"] || set[p+"basic-password-env"]
		if bearer && basic {
			return o, errors.New("bearer and basic auth cannot be combined")
		}
		if bearer {
			e.Auth = config.Auth{Type: "bearer", TokenEnv: f.bearer}
		}
		if basic {
			e.Auth = config.Auth{Type: "basic", UsernameEnv: f.user, PasswordEnv: f.password}
		}
		if set[p+"cookie-env"] {
			e.CookieEnv = f.cookie
		}
		if err = mergeHeaders(e, f.headers, f.envHeaders); err != nil {
			return o, err
		}
	}
	return o, o.Validate()
}
func reorder(args []string, fs *flag.FlagSet) (flags, pos []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
			continue
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		f := fs.Lookup(name)
		if f == nil {
			return nil, nil, errors.New("unknown option; see --help")
		}
		flags = append(flags, a)
		isBool := name == "help" || name == "h" || name == "version"
		if !hasValue && !isBool {
			i++
			if i >= len(args) {
				return nil, nil, errors.New("option requires a value")
			}
			flags = append(flags, args[i])
		}
	}
	return flags, pos, nil
}
func mergeHeaders(e *config.Endpoint, plain, env listFlag) error {
	// Canonicalize existing maps before applying CLI overrides across both sources.
	canonical := func(m map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range m {
			out[http.CanonicalHeaderKey(k)] = v
		}
		return out
	}
	// Validate first so duplicate case variants in a config cannot disappear during canonicalization.
	seen := map[string]bool{}
	for _, m := range []map[string]string{e.Headers, e.HeaderEnv} {
		for k := range m {
			n := http.CanonicalHeaderKey(k)
			if seen[n] {
				return errors.New("duplicate config request header")
			}
			seen[n] = true
		}
	}
	e.Headers, e.HeaderEnv = canonical(e.Headers), canonical(e.HeaderEnv)
	seen = map[string]bool{}
	for _, source := range []struct {
		items listFlag
		env   bool
	}{{plain, false}, {env, true}} {
		separator := ":"
		if source.env {
			separator = "="
		}
		for _, item := range source.items {
			name, value, ok := strings.Cut(item, separator)
			name = strings.TrimSpace(name)
			value = strings.TrimSpace(value)
			if !ok || !config.ValidHeaderName(name) {
				return errors.New("invalid header option")
			}
			name = http.CanonicalHeaderKey(name)
			if seen[name] {
				return errors.New("duplicate CLI request header")
			}
			seen[name] = true
			delete(e.Headers, name)
			delete(e.HeaderEnv, name)
			if source.env {
				e.HeaderEnv[name] = value
			} else {
				e.Headers[name] = value
			}
		}
	}
	return nil
}
