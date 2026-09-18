# 001-support-legacy-annotation.patch

Support extended-monitoring legacy annotation for now. Upstream project has an option to use label Namespace selector only.

### 002-go-mod-bump.patch

Update `golang.org/x/*` dependencies to fix CVEs.

* `golang.org/x/net` v0.38.0 -> v0.57.0 — closes CVE-2025-47911, CVE-2025-58190, CVE-2026-25680,
  CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506
  and CVE-2026-46600.
* `golang.org/x/sys` v0.42.0 -> v0.47.0 — closes CVE-2026-39824.
* `golang.org/x/text` v0.23.0 -> v0.40.0 — closes CVE-2026-56852.

Transitively required and included in the patch: `golang.org/x/sync` v0.20.0 -> v0.22.0,
`golang.org/x/term` v0.30.0 -> v0.45.0 and the matching `go.sum` updates. The `go` directive is left at
the upstream value `1.25.8`: the module graph of v0.16.0 already requires at least Go 1.25.7, and the
builder image `builder/golang-alpine` provides Go 1.25.13.

CVE-2025-15558 (`github.com/docker/cli`) is not covered by this patch — it is closed by the upstream
bump 0.14.0 -> 0.16.0 in `werf.inc.yaml`, which brings `github.com/docker/cli` v29.3.0+incompatible.

Generated on a clone of upstream `v0.16.0` with `001-support-legacy-annotation.patch` applied and
committed as the baseline:

```
go get golang.org/x/net@v0.57.0 golang.org/x/sys@v0.47.0 golang.org/x/text@v0.40.0 golang.org/x/term@v0.45.0 golang.org/x/sync@v0.22.0
go mod tidy
git diff -- go.mod go.sum
```
