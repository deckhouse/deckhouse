# Patches

## go-mod.patch

Updates `go.mod`/`go.sum` of CoreDNS: raises the Go directive and bumps a set of
dependencies.

Bumps the `golang.org/x/*` family, moving `golang.org/x/mod` from `v0.37.0` to
`v0.40.0` to remediate the following vulnerabilities:

- CVE-2026-56864 (`golang.org/x/mod`)
- CVE-2026-56865 (`golang.org/x/mod`)
