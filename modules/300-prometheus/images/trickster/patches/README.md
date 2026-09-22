## Patches

### 001-go-mod.patch

Update dependencies.

The `go` directive is raised to `1.25.0` — the highest version the DKP builder image supports
(`candi/base_images.yml`, `builder/golang-alpine` → `builder/golang-alpine-1.25`). It is required by
the dependencies bumped in `002-cve-go-mod.patch`.

### 002-cve-go-mod.patch

Single cumulative `go.mod`/`go.sum` bump on top of `001-go-mod.patch` that closes every CVE found in
the vendored `golang.org/x/*` packages.

Direct fixes:

| Dependency | Version | CVEs |
|---|---|---|
| `golang.org/x/net` | v0.38.0 → v0.56.0 | [CVE-2025-47911](https://nvd.nist.gov/vuln/detail/CVE-2025-47911), [CVE-2025-58190](https://nvd.nist.gov/vuln/detail/CVE-2025-58190), [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506), [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) |
| `golang.org/x/sys` | v0.31.0 → v0.46.0 | [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — also the minimum version `golang.org/x/net` v0.56.0 requires via MVS |

`go mod tidy` additionally refreshed `golang.org/x/text` in `go.sum` from v0.23.0 to v0.38.0
(transitive, not a direct requirement of Trickster). `golang.org/x/crypto` stays at v0.53.0, which
is what the resolved module graph selects — well under the v0.55.0 ceiling imposed by the Go 1.25.x
builder image. No source changes are needed and `go build ./...` succeeds.

`github.com/apache/thrift` stays at v0.13.0: the fixes for CVE-2026-41602 (v0.23.0) and
CVE-2026-43871 (v0.24.0) require a Thrift API generation that the vendored
`go.opentelemetry.io/otel/exporters/trace/jaeger` v0.16.0 generated code does not compile against.
Both findings are covered by `known_vulnerabilities.vex` for this image.

Generated with:

```sh
git clone --depth 1 --branch v1.1.5 https://github.com/trickstercache/trickster.git .
git apply /path/to/001-go-mod.patch --verbose
git add -A && git commit -m "baseline"
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go get golang.org/x/net@v0.56.0 golang.org/x/sys@v0.46.0
go mod tidy
go build ./...
git diff HEAD -- go.mod go.sum > 002-cve-go-mod.patch
```
