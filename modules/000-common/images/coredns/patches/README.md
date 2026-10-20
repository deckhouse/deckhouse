# Patches

## go-mod.patch

Updates `go.mod`/`go.sum` of CoreDNS: raises the Go directive and bumps a set of
dependencies.

Bumps the `golang.org/x/*` family, moving `golang.org/x/mod` from `v0.37.0` to
`v0.40.0` to remediate the following vulnerabilities:

- CVE-2026-56864 (`golang.org/x/mod`)
- CVE-2026-56865 (`golang.org/x/mod`)

Raises the Go directive to `1.26.0` (required by `golang.org/x/crypto` `v0.56.0`),
moves `golang.org/x/crypto` from `v0.55.0` to `v0.56.0` and `google.golang.org/grpc`
from `v1.82.1` to `v1.83.2` to remediate the following vulnerabilities:

- CVE-2026-56855 (`golang.org/x/crypto`)
- CVE-2026-78662 (`golang.org/x/crypto`)
- CVE-2026-84303 (`google.golang.org/grpc`)
- CVE-2026-84304 (`google.golang.org/grpc`)
- CVE-2026-84445 (`google.golang.org/grpc`)

Moves `golang.org/x/net` from `v0.58.0` to `v0.60.0` (pulling `golang.org/x/crypto`
`v0.57.0`, `golang.org/x/sys` `v0.48.0`, `golang.org/x/text` `v0.42.0`,
`golang.org/x/term` `v0.46.0`, `golang.org/x/sync` `v0.23.0` and `golang.org/x/mod`
`v0.41.0`) to remediate the following vulnerabilities:

- CVE-2026-78659 (`golang.org/x/net`)
- CVE-2026-78660 (`golang.org/x/net`)
- CVE-2026-78663 (`golang.org/x/net`)
- CVE-2026-78669 (`golang.org/x/net`)
- CVE-2026-97032 (`golang.org/x/net`)
