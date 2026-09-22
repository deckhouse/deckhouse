## Patches

### 001-go-mod.patch

Single cumulative `go.mod`/`go.sum` patch applied on top of the pristine
`prometheus/memcached_exporter` v0.15.3 tree. It raises the `go` directive to `1.25.0` — the highest
version the DKP builder image supports (`candi/base_images.yml`, `builder/golang-alpine` →
`builder/golang-alpine-1.25`) — and closes every CVE found in the exporter's Go dependencies.

Direct fixes:

| Dependency | Version | CVEs |
|---|---|---|
| `golang.org/x/crypto` | v0.38.0 → v0.55.0 | [CVE-2025-47913](https://github.com/advisories/GHSA-h4h4-33j5-h9f9), [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv), [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x), [CVE-2026-39827](https://github.com/advisories/GHSA-qpw4-5x99-6vjp), [CVE-2026-39828](https://github.com/advisories/GHSA-45gg-vh54-h5m9), [CVE-2026-39829](https://github.com/advisories/GHSA-w879-237q-wc7r), [CVE-2026-39830](https://github.com/advisories/GHSA-vgwf-h737-ff37), [CVE-2026-39831](https://github.com/advisories/GHSA-89gr-r52h-f8rx), [CVE-2026-39832](https://github.com/advisories/GHSA-f5wc-c3c7-36mc), [CVE-2026-39833](https://github.com/advisories/GHSA-jppx-rxg9-jmrx), [CVE-2026-39834](https://github.com/advisories/GHSA-rm3j-f69w-wqmq), [CVE-2026-39835](https://github.com/advisories/GHSA-78mq-xcr3-xm33), [CVE-2026-42508](https://github.com/advisories/GHSA-5cgq-3rg8-m6cv), [CVE-2026-46595](https://github.com/advisories/GHSA-x527-x647-q7gg), [CVE-2026-46597](https://github.com/advisories/GHSA-q4h4-gmj2-qvw2), [CVE-2026-46598](https://github.com/advisories/GHSA-9m57-25v3-79x9), [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854) |
| `golang.org/x/net` | v0.40.0 → v0.57.0 | [CVE-2025-47911](https://github.com/advisories/GHSA-8v43-wh42-33w6), [CVE-2025-58190](https://github.com/advisories/GHSA-93cg-qpqj-2r92), [CVE-2026-25680](https://github.com/advisories/GHSA-5cv4-jp36-h3mw), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506), [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) |
| `golang.org/x/sys` | v0.33.0 → v0.47.0 | [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) |
| `golang.org/x/text` | v0.25.0 → v0.41.0 | [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) |

Pulled in transitively via MVS: `golang.org/x/sync` v0.14.0 → v0.22.0.

**`golang.org/x/crypto` v0.55.0 is the ceiling**: v0.56.0 declares `go 1.26.0` in its own `go.mod`,
while the builder image ships Go 1.25.x. The two remaining `golang.org/x/crypto/ssh` findings that
require v0.56.0 (CVE-2026-56855, CVE-2026-78662) are handled by `known_vulnerabilities.vex`
instead, together with GO-2026-5932. Every module bumped here still declares `go 1.25.0`, so the
`go` directive of `memcached_exporter` stays at `1.25.0` and no `toolchain` line is introduced.

Generated with:

```sh
git clone --depth 1 --branch v0.15.3 https://github.com/prometheus/memcached_exporter.git .
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go mod edit -go=1.25.0
go get golang.org/x/crypto@v0.55.0 \
       golang.org/x/net@v0.57.0 \
       golang.org/x/sys@v0.47.0 \
       golang.org/x/text@v0.41.0
go mod tidy
go build ./...
git diff -- go.mod go.sum > 001-go-mod.patch
```
