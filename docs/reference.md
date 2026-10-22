# CLI reference

```text
reqdiff [options] <left-url> <right-url>
reqdiff --config FILE [options]
```

The left response is the reference. `+` means present only on the right; `-`
means missing on the right; `~` means changed. This direction is fixed regardless
of which environment you call staging or production.

## Options

| Flag | Default | Meaning |
|---|---|---|
| `--mode` | `structure` | `structure` or `values` |
| `--format` | `text` | `text` or `json` |
| `--config` | none | Explicit JSON config file |
| `--timeout` | `10s` | Total timeout per endpoint, including reading the body |
| `--max-body-bytes` | `10485760` | Maximum decompressed body size per endpoint |
| `--ignore` | none | Repeated JSON Pointer patterns; skip entire subtrees |
| `--ignore-value` | none | Repeated patterns; skip values but keep types and presence |
| `--compare-header` | `Content-Type` | Repeated response header names |
| `--max-latency-regression` | none | Fail if `right - left` exceeds this duration |
| `--help`, `-h` | | Show help without making requests |
| `--version` | | Show build version |

Per-endpoint options (replace `left` with `right` for the other endpoint):

| Flag | Meaning |
|---|---|
| `--left-bearer-env NAME` | Bearer token environment variable name |
| `--left-basic-user-env NAME` | Basic username environment variable name |
| `--left-basic-password-env NAME` | Basic password environment variable name |
| `--left-header 'Name: value'` | Repeated non-secret header |
| `--left-header-env 'Name=ENV'` | Repeated header backed by a secret environment variable |
| `--left-cookie-env NAME` | Environment variable containing the Cookie header |

A missing or empty credential variable fails validation before either request.
Bearer and Basic cannot be combined on the same side. Cookie can accompany
Bearer or Basic. Basic usernames cannot contain a colon.

Header names are case-insensitive. Repeating a name within CLI headers or within
config header sources is an error. `Authorization` and `Cookie` cannot be supplied
as plain `--left-header` values. Use their auth flags or `header-env`.
`Host`, `Content-Length`, `Transfer-Encoding`, `Connection`, `Proxy-Authorization`,
and `Accept-Encoding` are reserved. `Accept: application/json` is sent by default
and can be overridden.

## Config precedence

See the [example](../examples/reqdiff.json) for the schema. Unknown properties,
duplicate object keys, trailing JSON, invalid durations, and invalid URLs are
rejected. The file size limit is 1 MiB. There is no `${...}` interpolation.

1. Built-in defaults.
2. Explicit config file.
3. Explicit CLI options.

Two positional URLs override both config URLs. One positional URL is invalid.
CLI list flags replace the corresponding config lists. CLI auth flags replace
the entire auth object for that side; an incomplete Basic override is an error.
CLI header names override the same config header name across both plain and env
sources. Auth/header conflicts in the merged result are rejected.

## JSON comparison

- Object key order is ignored; missing is different from `null`.
- Arrays compare by index. A longer array adds elements even in structure mode.
- Structure mode checks presence and types, not primitive values.
- Values mode also compares primitive values and array order.
- An added or removed subtree produces one difference at its topmost path.
- `1`, `1.0`, and `1e0` are numerically equal; numbers never pass through `float64`.
- A non-empty response must contain exactly one valid JSON value. Duplicate keys
  are rejected. Content-Type is compared separately; JSON with a wrong but valid
  media type can still be parsed and compared.
- Empty bodies are distinct from JSON `null`. Only zero-byte bodies count as
  empty; whitespace-only bodies are invalid JSON.
- JSON limits: 128 nested containers; 1024 characters per number token; absolute
  exponent at most 10000. Limits produce a controlled error rather than a diff.

### Ignore paths

Paths are JSON Pointer patterns: `/users/0/id`. Root is the empty string `''`.
`~1` encodes a literal `/`; `~0` encodes a literal `~`. A full `*` segment matches
one object key or array index. No recursive globbing, filters, or literal-star
escape is provided. Valid patterns that match nothing are allowed.

`--ignore` skips a matched subtree, including presence and type changes.
`--ignore-value` suppresses primitive values within a matched subtree, preserving
key/index presence and type checks. Ignored array elements keep their indexes.

## Headers and latency

Only selected response headers affect the comparison. Content-Type media type
and parameter names are normalized; parameter order is ignored, parameter values
are retained. Invalid Content-Type returns an error. Other headers trim outer
whitespace and preserve multi-value ordering; values are not split at commas.
`Date`, `Set-Cookie`, and request IDs are not included unless explicitly selected.
Cookie and Authorization header values are masked even when explicitly selected.

Latency starts just before sending the GET and ends after reading and closing the
body. JSON parsing and comparison are excluded. There is no warm-up or averaging.
A threshold of `150ms` is exceeded by `151ms`, not by `150ms` or by an improvement.

## Output and errors

Text reports go to stdout on success or differences; operational-error reports
go to stderr. JSON mode writes one report to stdout, including errors. If the
output stream itself fails, a short diagnostic goes to stderr and exit is `2`.
Terminal control characters from external data are escaped.

JSON report fields:

| Field | Meaning |
|---|---|
| `schema_version` | Currently `1` |
| `mode` | Selected body comparison mode |
| `result` | `equal`, `different`, `error`, or `cancelled` |
| `left`, `right` | Safe URL, nullable status, `duration_ns`, `body_bytes`, `body_kind` |
| `differences` | Ordered objects with `scope`, `path`, `kind`, types and optional primitive values |
| `latency` | `left_ns`, `right_ns`, `delta_ns`, optional `threshold_ns`, `exceeded` |
| `warnings` | Side, code and safe message; HTTP errors produce `http_error_status` |
| `errors` | Side, code and safe message |

Difference scopes: `body`, `status`, `header`. Kinds: `added`, `removed`,
`type_changed`, `value_changed`. Body paths use JSON Pointer; root path is `""`.
Header paths are header names. Missing sides omit their value property; JSON null
is an explicit value. Object/array subtrees are not dumped into value properties.
Header values are represented as JSON-array strings to preserve multiple values.

Error codes: `usage`, `config`, `auth`, `network`, `timeout`, `body_too_large`,
`invalid_json`, `limit_exceeded`, `cancelled`. Invalid Content-Type uses
`invalid_json`, reflecting an unusable JSON-response comparison. Failed output
uses exit `2` with a stderr diagnostic because the report cannot be written.

Operational errors take precedence over differences. Global context cancellation
takes precedence over operational errors. A failed side does not automatically
cancel the other side. Available status and timing are retained in partial
reports. An unavailable status is `null`; its body kind may be empty/unknown.

A `401`/`500` is an HTTP response, not a network failure. Two equal error responses
can exit `0` with warnings. Equality describes the selected comparison rules and
does not assert that either API is healthy.
