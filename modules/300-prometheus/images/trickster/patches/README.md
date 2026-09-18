## Patches

## 001-go-mod.patch

Update dependencies

The `go` directive is raised to `1.25.0` — the highest version the DKP builder image supports
(`candi/base_images.yml`, `builder/golang-alpine` → `builder/golang-alpine-1.25`). It is required by
the dependencies bumped in the later `*-fix-cve*-bump.patch` patches.

## 002-fix-cve-bump.patch

Bump dependencies to fix CVEs found in the vendored `golang.org/x/net` and `golang.org/x/sys` packages:

- [CVE-2025-47911](https://nvd.nist.gov/vuln/detail/CVE-2025-47911), [CVE-2025-58190](https://nvd.nist.gov/vuln/detail/CVE-2025-58190), [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506) — `golang.org/x/net` bumped from v0.38.0 to v0.55.0.
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — `golang.org/x/sys` bumped from v0.31.0 to v0.45.0 (the minimum version compatible with `golang.org/x/net` v0.55.0 via MVS).

`go mod tidy` also pulled a transitive bump of `golang.org/x/text` from v0.23.0 to v0.37.0.
No source changes were needed and `go build ./...` succeeds.

Generated with:

```sh
git clone --depth 1 --branch v1.1.5 https://github.com/trickstercache/trickster.git .
git apply /patches/001-go-mod.patch --verbose
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go get golang.org/x/net@v0.55.0 golang.org/x/sys@v0.45.0
go mod tidy
go build ./...
git diff HEAD -- go.mod go.sum > 002-fix-cve-bump.patch
```

## 003-fix-cve-bump.patch

Bump `golang.org/x/net` to fix a CVE found in the vendored package:

- [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) — `golang.org/x/net` bumped from v0.55.0 to v0.56.0.

`go mod tidy` also pulled transitive bumps required by the new `golang.org/x/net` requirement via MVS:
`golang.org/x/sys` v0.45.0 → v0.46.0 and `golang.org/x/text` v0.37.0 → v0.38.0.

`github.com/apache/thrift` stays at v0.13.0: the fixes for CVE-2026-41602 (v0.23.0) and
CVE-2026-43871 (v0.24.0) require a Thrift API generation that the vendored
`go.opentelemetry.io/otel/exporters/trace/jaeger` v0.16.0 generated code does not compile against.
Both findings are covered by `known_vulnerabilities.vex` for this image.

Generated with:

```sh
git clone --depth 1 --branch v1.1.5 https://github.com/trickstercache/trickster.git .
git apply /patches/001-go-mod.patch --verbose
git apply /patches/002-fix-cve-bump.patch --verbose
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go get golang.org/x/net@v0.56.0
go mod tidy
go build ./...
git diff HEAD -- go.mod go.sum > 003-fix-cve-bump.patch
```
