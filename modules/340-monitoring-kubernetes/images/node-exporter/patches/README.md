## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-52881](https://github.com/advisories/GHSA-cgrx-mc8f-2prm)
- CVE-2026-39827, CVE-2026-39828, CVE-2026-39829, CVE-2026-39830, CVE-2026-39831, CVE-2026-39832,
  CVE-2026-39833, CVE-2026-39834, CVE-2026-39835, CVE-2026-42508, CVE-2026-46595, CVE-2026-46597,
  CVE-2026-46598 — bump `golang.org/x/crypto` (advisory requires `0.52.0`)
- CVE-2026-56854 (`GO-2026-6303`, fixed in `0.55.0`) — bump `golang.org/x/crypto` to `v0.55.0`.
  **`v0.55.0` is the ceiling here**: `v0.56.0` declares `go 1.26.0`, while the image is built by
  `builder/golang-alpine`, which is pinned to Go 1.25 in `candi/base_images.yml`
- CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502,
  CVE-2026-42506, CVE-2026-46600 — bump `golang.org/x/net` (advisory requires `0.56.0`;
  `golang.org/x/crypto v0.55.0` pulls `0.58.0`)
- CVE-2026-56852 — bump `golang.org/x/text` (advisory requires `0.39.0`;
  `golang.org/x/crypto v0.55.0` pulls `0.41.0`)
- CVE-2026-39824 — bump `golang.org/x/sys` (advisory requires `0.44.0`;
  `golang.org/x/crypto v0.55.0` pulls `0.47.0`)

The `go` directive is raised to `go 1.25.0` to match the builder (`builder/golang-alpine` = Go 1.25).

The patch is regenerated on top of the pristine `v1.9.1` tag with:

```bash
git clone --depth 1 --branch v1.9.1 https://github.com/prometheus/node_exporter.git
cd node_exporter
sed -i 's/^go 1\.23\.0$/go 1.25.0/' go.mod
go get github.com/opencontainers/selinux@v1.13.0 \
  golang.org/x/crypto@v0.55.0 golang.org/x/net@v0.58.0 \
  golang.org/x/sys@v0.47.0 golang.org/x/text@v0.41.0
go mod tidy
git diff -- go.mod go.sum > 001-go-mod.patch
```

`GO-2026-5932` (`golang.org/x/crypto/openpgp` unmaintained), `CVE-2026-56855` and `CVE-2026-78662`
(both `golang.org/x/crypto/ssh`, fixed only in `v0.56.0`, which needs Go 1.26) have no reachable
fix in this branch and are handled by `../known_vulnerabilities.vex` instead — neither `ssh` nor
`openpgp` is in the binary's build graph.
