### 001-go-mod.patch

Update dependencies

### 002-go-mod.patch

Update `golang.org/x/*` dependencies and the `go` directive to fix CVEs.

* `golang.org/x/crypto` v0.47.0 -> v0.55.0 — closes CVE-2024-45337, CVE-2026-39827, CVE-2026-39828,
  CVE-2026-39829, CVE-2026-39830, CVE-2026-39831, CVE-2026-39832, CVE-2026-39833, CVE-2026-39834,
  CVE-2026-39835, CVE-2026-42508, CVE-2026-46595, CVE-2026-46597, CVE-2026-46598 and CVE-2026-56854.
  v0.55.0 is the ceiling for this branch: v0.56.0 declares `go 1.26.0`, while the builder image
  `builder/golang-alpine` provides Go 1.25.13 (`candi/base_images.yml` has no 1.26 key).
* `golang.org/x/net` v0.48.0 -> v0.57.0 — closes CVE-2026-25680, CVE-2026-25681, CVE-2026-27136,
  CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506 and CVE-2026-46600.
* `golang.org/x/sys` v0.40.0 -> v0.47.0 — closes CVE-2026-39824.
* `golang.org/x/text` v0.33.0 -> v0.41.0 — closes CVE-2026-56852. v0.41.0 (not v0.40.0) is required by
  `golang.org/x/crypto` v0.55.0.

Transitively required and included in the patch: `golang.org/x/sync` v0.19.0 -> v0.22.0,
`golang.org/x/term` v0.39.0 -> v0.45.0, the `go` directive 1.24.0 -> 1.25.0 and the matching `go.sum`
updates.

CVE-2026-56855, CVE-2026-78662 and GO-2026-5932 remain open in the scan and are waived in
`../known_vulnerabilities.vex`: they live in `golang.org/x/crypto/ssh` and `golang.org/x/crypto/openpgp`,
neither of which is part of the binary's import graph (only `bcrypt` and `blowfish` are linked).

Generated on a clone of upstream `v3.19.1` with `001-go-mod.patch` applied and committed as the
baseline:

```
go mod edit -go=1.25.0
go get golang.org/x/crypto@v0.55.0 golang.org/x/net@v0.57.0 golang.org/x/sys@v0.47.0 golang.org/x/text@v0.41.0 golang.org/x/term@v0.45.0 golang.org/x/sync@v0.22.0
go mod tidy
git diff -- go.mod go.sum
```
