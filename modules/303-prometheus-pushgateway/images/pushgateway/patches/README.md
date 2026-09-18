# Patches

Patches applied to the upstream `prometheus/pushgateway` source (tag `v1.11.3`)
during the `-src-artifact` build stage via `git apply /patches/*.patch`.

### 001-cve-go-mod.patch

Bump `golang.org/x/net` to v0.56.0 (fixes CVE-2026-46600 / GO-2026-5942: panic
parsing an invalid SVCB/HTTPS RR in `dns/dnsmessage`) and `golang.org/x/text` to
v0.39.0 (fixes CVE-2026-56852 / GO-2026-5970: infinite loop in `norm.Iter` on
invalid UTF-8). Required transitive bumps to `golang.org/x/crypto` (v0.53.0),
`golang.org/x/sync` (v0.21.0), and `golang.org/x/sys` (v0.46.0), plus the
matching `go.sum` updates, are included. Generated offline with
`go get golang.org/x/net@v0.56.0 golang.org/x/text@v0.39.0 && go mod tidy`.

### 002-cve-go-mod.patch

Bump `golang.org/x/crypto` to v0.55.0 (fixes CVE-2026-46595 and CVE-2026-56854,
reported against the v0.53.0 introduced by `001-cve-go-mod.patch`: source-address
restrictions returned by SSH authentication callbacks were not enforced;
CVE-2026-56854 extends the incomplete fix for CVE-2026-46595 to the password,
keyboard-interactive, no-client-auth, and GSSAPI callbacks). Required transitive
bumps to `golang.org/x/net` (v0.57.0), `golang.org/x/sync` (v0.22.0),
`golang.org/x/sys` (v0.47.0), and `golang.org/x/text` (v0.41.0), plus the
matching `go.sum` updates, are included. The `go` directive stays at 1.25.0.
Generated offline with `go get golang.org/x/crypto@v0.55.0 && go mod tidy`.

Note: `golang.org/x/crypto` is deliberately capped at v0.55.0. v0.56.0 declares
`go 1.26.0`, while the builder image used here (`builder/golang-alpine` in
`candi/base_images.yml`) ships Go 1.25.13, so v0.56.0 cannot be compiled in this
repository. The two findings that are only fixed in v0.56.0 — CVE-2026-56855 /
GO-2026-6355 and CVE-2026-78662 / GO-2026-6354, a denial of service on a
deadlocked established/undecided channel in `golang.org/x/crypto/ssh` — are
covered by `known_vulnerabilities.vex` instead: `/bin/pushgateway` does not link
`golang.org/x/crypto/ssh`.

The four CVEs reported against `github.com/prometheus/prometheus`
(CVE-2026-40179, CVE-2026-42151, CVE-2026-42154, CVE-2026-44903) are fixed by the
upstream tag bump from v1.11.1 to v1.11.3, which carries
`github.com/prometheus/prometheus` v0.311.3.
