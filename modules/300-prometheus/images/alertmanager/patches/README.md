## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-47908](https://github.com/advisories/GHSA-mh55-gqvf-xfwm)

The `go` directive is raised to `1.25.0` — the highest version the DKP builder image supports
(`candi/base_images.yml`, `builder/golang-alpine` → `builder/golang-alpine-1.25`). It is required by
the dependencies bumped in the later `*-fix-cve*-bump.patch` patches.

### 002-fix-cve-bump.patch

Bump dependencies to fix CVEs:
- [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303) — go.mongodb.org/mongo-driver
- [CVE-2026-39827](https://github.com/advisories/GHSA-qpw4-5x99-6vjp) — golang.org/x/crypto/ssh
- [CVE-2026-39828](https://github.com/advisories/GHSA-45gg-vh54-h5m9) — golang.org/x/crypto/ssh
- [CVE-2026-39829](https://github.com/advisories/GHSA-w879-237q-wc7r) — golang.org/x/crypto/ssh
- [CVE-2026-39830](https://github.com/advisories/GHSA-vgwf-h737-ff37) — golang.org/x/crypto/ssh
- [CVE-2026-39831](https://github.com/advisories/GHSA-89gr-r52h-f8rx) — golang.org/x/crypto/ssh
- [CVE-2026-39832](https://github.com/advisories/GHSA-f5wc-c3c7-36mc) — golang.org/x/crypto/ssh
- [CVE-2026-39833](https://github.com/advisories/GHSA-jppx-rxg9-jmrx) — golang.org/x/crypto/ssh
- [CVE-2026-39834](https://github.com/advisories/GHSA-rm3j-f69w-wqmq) — golang.org/x/crypto/ssh
- [CVE-2026-39835](https://github.com/advisories/GHSA-78mq-xcr3-xm33) — golang.org/x/crypto/ssh
- [CVE-2026-42508](https://github.com/advisories/GHSA-5cgq-3rg8-m6cv) — golang.org/x/crypto/ssh
- [CVE-2026-46595](https://github.com/advisories/GHSA-x527-x647-q7gg) — golang.org/x/crypto/ssh
- [CVE-2026-46597](https://github.com/advisories/GHSA-q4h4-gmj2-qvw2) — golang.org/x/crypto/ssh
- [CVE-2026-46598](https://github.com/advisories/GHSA-9m57-25v3-79x9) — golang.org/x/crypto/ssh
- [CVE-2026-25680](https://github.com/advisories/GHSA-5cv4-jp36-h3mw) — golang.org/x/net/http2
- [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681) — golang.org/x/net/html
- [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136) — golang.org/x/net/html
- [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814) — golang.org/x/net (HTTP/2 transport)
- [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821) — golang.org/x/net/idna
- [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502) — golang.org/x/net/html
- [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506) — golang.org/x/net/html
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — golang.org/x/sys/windows

Generated with:

```sh
go get go.mongodb.org/mongo-driver@v1.17.7 golang.org/x/crypto@v0.52.0 golang.org/x/net@v0.55.0 golang.org/x/sys@v0.45.0
go mod tidy
```

### 003-fix-cve-bump.patch

Bump dependencies to fix CVEs:
- [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) — golang.org/x/net
- [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) — golang.org/x/text

Notable transitive bumps: `golang.org/x/crypto` v0.52.0 → v0.53.0, `golang.org/x/sync` v0.20.0 → v0.21.0, `golang.org/x/sys` v0.45.0 → v0.46.0.

Generated with:

```sh
go get golang.org/x/net@v0.56.0 golang.org/x/text@v0.39.0
go mod tidy
```

### 004-fix-cve-bump.patch

Bump `golang.org/x/crypto` from v0.53.0 to v0.55.0:
- [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595) and [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854) — source-address restrictions returned by SSH authentication callbacks were not enforced.

v0.55.0 is the ceiling for DKP 1.76: `golang.org/x/crypto` v0.56.0 declares `go 1.26.0` in its own
`go.mod`, while the builder image (`candi/base_images.yml`, `builder/golang-alpine` →
`builder/golang-alpine-1.25`) ships Go 1.25.x. The two remaining `golang.org/x/crypto/ssh` findings
that would require v0.56.0 (CVE-2026-56855, CVE-2026-78662) are handled by
`known_vulnerabilities.vex` instead, together with GO-2026-5932 (`golang.org/x/crypto/openpgp`).

Transitive bumps that come with v0.55.0 via MVS: `mod` v0.37.0 → v0.38.0, `net` v0.56.0 → v0.57.0,
`sync` v0.21.0 → v0.22.0, `sys` v0.46.0 → v0.47.0, `text` v0.39.0 → v0.41.0, `tools` v0.47.0 →
v0.48.0, `telemetry`. All of them still declare `go 1.25.0`, so the `go` directive of Alertmanager
stays at `1.25.0`.

Generated with:

```sh
git clone --depth 1 --branch v0.27.0 https://github.com/prometheus/alertmanager.git .
git apply /path/to/001-go-mod.patch --verbose
git apply /path/to/002-fix-cve-bump.patch --verbose
git apply /path/to/003-fix-cve-bump.patch --verbose
git add -A && git commit -m "post-existing-patches baseline"
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go get golang.org/x/crypto@v0.55.0
go mod tidy
go build ./cmd/alertmanager
git diff HEAD -- go.mod go.sum > 004-fix-cve-bump.patch
```
