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
