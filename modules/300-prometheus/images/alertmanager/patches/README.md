## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-47908](https://github.com/advisories/GHSA-mh55-gqvf-xfwm)

The `go` directive is raised to `1.25.0` — the highest version the DKP builder image supports
(`candi/base_images.yml`, `builder/golang-alpine` → `builder/golang-alpine-1.25`). It is required by
the dependencies bumped in `002-cve-go-mod.patch`.

### 002-cve-go-mod.patch

Single cumulative `go.mod`/`go.sum` bump on top of `001-go-mod.patch` that closes every remaining
CVE found in Alertmanager's Go dependencies.

Direct fixes:

| Dependency | Version | CVEs |
|---|---|---|
| `go.mongodb.org/mongo-driver` | v1.13.1 → v1.17.7 | [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303) |
| `golang.org/x/crypto` | v0.47.0 → v0.55.0 | [CVE-2026-39827](https://github.com/advisories/GHSA-qpw4-5x99-6vjp), [CVE-2026-39828](https://github.com/advisories/GHSA-45gg-vh54-h5m9), [CVE-2026-39829](https://github.com/advisories/GHSA-w879-237q-wc7r), [CVE-2026-39830](https://github.com/advisories/GHSA-vgwf-h737-ff37), [CVE-2026-39831](https://github.com/advisories/GHSA-89gr-r52h-f8rx), [CVE-2026-39832](https://github.com/advisories/GHSA-f5wc-c3c7-36mc), [CVE-2026-39833](https://github.com/advisories/GHSA-jppx-rxg9-jmrx), [CVE-2026-39834](https://github.com/advisories/GHSA-rm3j-f69w-wqmq), [CVE-2026-39835](https://github.com/advisories/GHSA-78mq-xcr3-xm33), [CVE-2026-42508](https://github.com/advisories/GHSA-5cgq-3rg8-m6cv), [CVE-2026-46595](https://github.com/advisories/GHSA-x527-x647-q7gg), [CVE-2026-46597](https://github.com/advisories/GHSA-q4h4-gmj2-qvw2), [CVE-2026-46598](https://github.com/advisories/GHSA-9m57-25v3-79x9), [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854) |
| `golang.org/x/net` | v0.48.0 → v0.57.0 | [CVE-2026-25680](https://github.com/advisories/GHSA-5cv4-jp36-h3mw), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506), [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) |
| `golang.org/x/sys` | v0.40.0 → v0.47.0 | [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) |
| `golang.org/x/text` | v0.33.0 → v0.41.0 | [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) |

Pulled in transitively via MVS (no CVE of their own, required by the versions above):
`golang.org/x/mod` v0.31.0 → v0.38.0, `golang.org/x/sync` v0.19.0 → v0.22.0,
`golang.org/x/tools` v0.40.0 → v0.48.0, `golang.org/x/telemetry`.

**`golang.org/x/crypto` v0.55.0 is the ceiling for DKP 1.76**: v0.56.0 declares `go 1.26.0` in its
own `go.mod`, while the builder image (`candi/base_images.yml`, `builder/golang-alpine` →
`builder/golang-alpine-1.25`) ships Go 1.25.x. The two remaining `golang.org/x/crypto/ssh` findings
that would require v0.56.0 (CVE-2026-56855, CVE-2026-78662) are handled by
`known_vulnerabilities.vex` instead, together with GO-2026-5932 (`golang.org/x/crypto/openpgp`).
Everything bumped here still declares `go 1.25.0`, so Alertmanager's `go` directive stays at
`1.25.0` and no `toolchain` line is introduced.

Generated with:

```sh
git clone --depth 1 --branch v0.27.0 https://github.com/prometheus/alertmanager.git .
git apply /path/to/001-go-mod.patch --verbose
git add -A && git commit -m "baseline"
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go get go.mongodb.org/mongo-driver@v1.17.7 \
       golang.org/x/crypto@v0.55.0 \
       golang.org/x/net@v0.57.0 \
       golang.org/x/sys@v0.47.0 \
       golang.org/x/text@v0.41.0
go mod tidy
go build ./cmd/alertmanager
git diff HEAD -- go.mod go.sum > 002-cve-go-mod.patch
```
