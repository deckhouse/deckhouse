### 001-go-mod-logrus.patch

Force `github.com/sirupsen/logrus` to v1.9.3 via a `replace` directive in `go.mod` to fix CVE-2025-65637,
and drop the stale `logrus v1.2.0`, `v1.4.2` and `v1.6.0` `/go.mod` checksums from `go.sum`.

### 002-go-mod-bump.patch

Update `golang.org/x/*` dependencies and the `go` directive to fix CVEs.

* `golang.org/x/net` v0.38.0 -> v0.57.0 — closes CVE-2025-47911, CVE-2025-58190, CVE-2026-25680,
  CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506
  and CVE-2026-46600.
* `golang.org/x/sys` v0.31.0 -> v0.47.0 — closes CVE-2026-39824.
* `golang.org/x/text` v0.23.0 -> v0.40.0 — closes CVE-2026-56852.

Transitively required and included in the patch: `golang.org/x/term` v0.30.0 -> v0.45.0, the `go`
directive 1.24 -> 1.25.0 (the builder image `builder/golang-alpine` provides Go 1.25.13) and the
matching `go.sum` updates.

Generated on a clone of upstream `v0.0.5` with `001-go-mod-logrus.patch` applied and committed as the
baseline:

```
go get golang.org/x/net@v0.57.0 golang.org/x/sys@v0.47.0 golang.org/x/text@v0.40.0 golang.org/x/term@v0.45.0
go mod tidy
git diff -- go.mod go.sum
```
