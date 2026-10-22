# Contributing

Keep changes focused. Start with a test that demonstrates the behavior you want
to change, then make it pass. Prefer small functions and the standard library;
new dependencies should solve a concrete problem.

## Run checks

```sh
docker build --target test -t reqdiff:test .
```

Or with Go 1.26+:

```sh
go test ./...
go test -race ./...
go vet ./...
gofmt -w cmd internal
```

Tests use `httptest` HTTP/TLS servers. Do not introduce live API dependencies or
real credentials. Test observable behavior (requests, reports, exit codes) rather
than private implementation details. Channel-based synchronization is preferable
to timing-sensitive sleeps when testing concurrency.

For parser fuzzing:

```sh
go test ./internal/jsondoc -run '^$' -fuzz FuzzDecode -fuzztime 10s
```

## Project layout

`internal/cli` handles arguments and orchestration, `config` handles configuration,
`auth` resolves credentials, `httpfetch` owns network requests, `jsondoc` reads
strict JSON, `compare` produces differences, and `report` renders output.
`cmd/reqdiff` only wires signals, streams, and the process exit code.

Check the public
[reference](docs/reference.md) before changing defaults, exit codes, or the JSON
schema. Keep documentation and tests in the same change as behavior updates.

Short commit subjects are welcome: `feat: add ...`, `fix: handle ...`,
`test: cover ...`, `docs: explain ...`.

## Demo assets

Build `dist/reqdiff`, then run `python3 scripts/render_demo.py`. The renderer
requires Pillow (development-only) and captures a real comparison against the
local synthetic server. It writes `docs/assets/demo.gif`; its runtime dependency
is not part of reqdiff. Never use real customer output in screenshots.

## Before the first public release

Choose the GitHub repository URL, update the module path and internal imports
from `reqdiff` together if publishing a `go install` address, and verify the name
is suitable for your project. No repository URL or registry namespace is assumed
in this checkout. The code is licensed under MIT; confirm that choice before
publishing. Tagging `v0.1.0` after pushing the repository runs the release workflow.
