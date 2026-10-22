# MVP verification

Verified locally on 22 October 2026. This records what was actually exercised;
it does not claim a hosted GitHub Actions run.

## Automated checks

- 43 Go test functions and 7 subtests passed.
- `go test -race ./...` passed on macOS arm64 and in the Linux arm64 Docker test target.
- `go vet ./...`, formatting, and `git diff --check` passed.
- Statement coverage: 88.1% overall, including the thin process entrypoint.
- Parser fuzzing: 173,633 executions over an approximately 11-second run; no failures.
- CLI and HTTP package tests repeated ten times without failures before the final redaction fix; the complete suite and race tests were rerun after the fix.
- Docker tests use Go 1.26.0; local checks and release cross-compilation use Go 1.27.3.

Tests cover separate credentials, Basic Auth, cookies, header-env, preflight
validation with no requests, concurrent GETs, TLS, redirects, gzip/body limits,
request and body-read timeouts, cancellation, strict JSON, exact numbers, arrays,
ignored paths, headers, latency thresholds, safe reports, and output errors.

## Executable and packaging checks

- Native binary and Docker runtime both reported `reqdiff 0.1.0`.
- Both compared the local synthetic endpoints with the expected three differences and exit code `1`.
- A read-only config mount worked with the non-root Docker runtime.
- An actual SIGINT to the binary produced exit `130` and a valid cancelled JSON report.
- Linux, macOS, and Windows release binaries cross-compiled for amd64 and arm64.
- All six archives passed SHA-256 and exact-content checks using `scripts/check-release.py`.
- The native macOS arm64 release binary was run. Windows and the other CPU architectures were cross-compiled, not executed locally; the included CI matrix runs native OS tests once hosted.

## Independent review and fixes

A separate read-only review found one important issue: credentials containing
special characters could remain recoverable after JSON Pointer, header JSON,
or URL percent encoding. `TestRunRedactsEncodedCredentials` first reproduced the
leaks, then passed after sanitizing decoded values before display encoding.
Both text and JSON formats are covered; comparison still uses original values.
No critical or additional minor findings were reported.

Archive inspection separately found macOS AppleDouble metadata entries. The
archive verifier failed before the fix and passed after disabling that metadata
when creating tar archives.

## Implementation decisions

- Work stayed in the requested fresh directory on `feat/mvp`; there was no existing branch or user code to isolate in another worktree.
- Go was absent and was installed so real checks could run. Docker support was then added at the user's request; local Go is not required to build or test through Docker.
- Strict JSON resource checks were implemented together with the first decoder task rather than duplicating parsing in a later task. Their test and fuzz coverage remains separate.
- The initial local module was `reqdiff`; publication preparation changed it and all internal imports to `github.com/yusufalper/reqdiff`.
- The public distribution uses the MIT license.
- At the original local handoff there was no remote or release. The user subsequently created `yusufalper/reqdiff` and authorized publishing the code and first release. The checks above describe local verification; hosted workflow results are available in the repository Actions tab.
- No review findings were left deferred.
