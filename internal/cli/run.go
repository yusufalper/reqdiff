package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reqdiff/internal/auth"
	"reqdiff/internal/compare"
	"reqdiff/internal/httpfetch"
	"reqdiff/internal/report"
	"strings"
)

var Version = "dev"

func Run(ctx context.Context, args []string, stdout, stderr io.Writer, lookup func(string) (string, bool)) int {
	opts, err := Parse(args)
	if errors.Is(err, ErrHelp) {
		if _, err := io.WriteString(stdout, Usage); err != nil {
			return 2
		}
		return 0
	}
	if errors.Is(err, ErrVersion) {
		if _, err := fmt.Fprintln(stdout, "reqdiff "+Version); err != nil {
			return 2
		}
		return 0
	}
	format := opts.Format
	if wantsJSON(args) {
		format = "json"
	}
	r := report.New(opts.Mode)
	redact := auth.NewRedactor(nil)
	emit := func() int {
		code := 0
		switch r.Result {
		case "different":
			code = 1
		case "error":
			code = 2
		case "cancelled":
			code = 130
		}
		var writeErr error
		if format == "json" {
			writeErr = report.WriteJSON(stdout, r, redact.String)
		} else {
			w := stdout
			if code == 2 || code == 130 {
				w = stderr
			}
			writeErr = report.WriteText(w, r, redact.String)
		}
		if writeErr != nil {
			fmt.Fprintln(stderr, "reqdiff: cannot write report")
			return 2
		}
		return code
	}
	fail := func(side, code, message string) int {
		r.Result = "error"
		r.Errors = append(r.Errors, report.Error{Side: side, Code: code, Message: message})
		return emit()
	}
	if err != nil {
		return fail("global", "usage", err.Error())
	}
	if _, err = compare.CompilePaths(opts.Ignore); err != nil {
		return fail("global", "config", err.Error())
	}
	if _, err = compare.CompilePaths(opts.IgnoreValues); err != nil {
		return fail("global", "config", err.Error())
	}
	left, leftErr := auth.Resolve(opts.Left, lookup)
	right, rightErr := auth.Resolve(opts.Right, lookup)
	secrets := append(append([]string{}, left.Secrets...), right.Secrets...)
	redact = auth.NewRedactor(secrets)
	if leftErr != nil {
		return fail("left", "auth", leftErr.Error())
	}
	if rightErr != nil {
		return fail("right", "auth", rightErr.Error())
	}
	type fetched struct {
		side     int
		snapshot httpfetch.Snapshot
		err      error
	}
	results := make(chan fetched, 2)
	for i, e := range []auth.ResolvedEndpoint{left, right} {
		go func(side int, endpoint auth.ResolvedEndpoint) {
			client := httpfetch.NewClient(opts.Timeout)
			defer client.CloseIdleConnections()
			s, err := httpfetch.Fetch(ctx, client, endpoint, opts.MaxBodyBytes)
			results <- fetched{side, s, err}
		}(i, e)
	}
	var snapshots [2]httpfetch.Snapshot
	var fetchErrors [2]error
	for range 2 {
		v := <-results
		snapshots[v.side], fetchErrors[v.side] = v.snapshot, v.err
	}
	r, err = compare.Responses(snapshots[0], snapshots[1], opts)
	if err != nil && len(r.Errors) == 0 {
		r.Errors = append(r.Errors, report.Error{Side: "global", Code: "config", Message: err.Error()})
	}
	for i, err := range fetchErrors {
		if err == nil {
			continue
		}
		side := "left"
		if i == 1 {
			side = "right"
		}
		code := "network"
		var fetchErr *httpfetch.Error
		if errors.As(err, &fetchErr) {
			code = fetchErr.Code
		}
		r.Errors = append(r.Errors, report.Error{Side: side, Code: code, Message: err.Error()})
	}
	if len(r.Errors) > 0 {
		r.Result = "error"
	}
	if ctx.Err() != nil {
		r.Result = "cancelled"
		r.Errors = append(r.Errors, report.Error{Side: "global", Code: "cancelled", Message: "comparison cancelled"})
	}
	return emit()
}

// Recover the requested output format even when another option is invalid.
func wantsJSON(args []string) bool {
	result := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, value, inline := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name == "help" || name == "h" || name == "version" {
			continue
		}
		if !inline && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			i++
			value = args[i]
		}
		if name == "format" {
			result = value == "json"
		}
	}
	return result
}
