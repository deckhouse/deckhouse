# Patches

Patches applied to the upstream `deckhouse/events_exporter` source (tag `v0.0.5`)
during the `-src-artifact` build stage via `git apply /patches/*.patch`.

### 001-go-mod.patch

Update dependencies and the `go` directive to fix CVEs. Only `go.mod` and `go.sum`
are touched.

* `github.com/sirupsen/logrus` pinned to v1.9.3 via a `replace` directive — closes
  CVE-2025-65637. The stale `logrus` v1.2.0, v1.4.2 and v1.6.0 `/go.mod` checksums
  are dropped from `go.sum` accordingly.
* `golang.org/x/net` v0.38.0 -> v0.57.0 — closes CVE-2025-47911, CVE-2025-58190,
  CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821,
  CVE-2026-42502, CVE-2026-42506 and CVE-2026-46600.
* `golang.org/x/sys` v0.31.0 -> v0.47.0 — closes CVE-2026-39824.
* `golang.org/x/text` v0.23.0 -> v0.40.0 — closes CVE-2026-56852.

Transitively required and included in the patch: `golang.org/x/term` v0.30.0 -> v0.45.0,
the `go` directive 1.24 -> 1.25.0 (the builder image `builder/golang-alpine` provides
Go 1.25.13) and the matching `go.sum` updates.

Generated on a pristine clone of upstream `v0.0.5`:

```
go mod edit -replace github.com/sirupsen/logrus=github.com/sirupsen/logrus@v1.9.3 -go=1.25.0
go get golang.org/x/net@v0.57.0 golang.org/x/sys@v0.47.0 golang.org/x/text@v0.40.0 golang.org/x/term@v0.45.0
go mod tidy
git diff -- go.mod go.sum
```
