## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 003-cve-go-mod.patch

Update Go dependencies to fix CVEs
- [CVE-2026-33186](https://github.com/advisories/GHSA-p77j-4mvh-x3m3) — `google.golang.org/grpc` bumped to `v1.80.0`
- [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq) — `go.opentelemetry.io/otel/sdk` bumped to `v1.43.0`
- [CVE-2026-39883](https://github.com/advisories/GHSA-hfvc-g4fc-pqhx) — `go.opentelemetry.io/otel/sdk` bumped to `v1.43.0`
- [CVE-2026-39882](https://github.com/advisories/GHSA-w8rr-5gcm-pp58) — `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` bumped to `v1.43.0`
- [CVE-2026-33487](https://github.com/advisories/GHSA-479m-364c-43vc) — `github.com/russellhaering/goxmldsig` bumped to `v1.6.0`
- [CVE-2026-34986](https://github.com/advisories/GHSA-78h2-9frx-2jm8) — `github.com/go-jose/go-jose/v3` bumped to `v3.0.5`
- [CVE-2026-1229](https://github.com/advisories/GHSA-q9hv-hpm4-hj6x) — `github.com/cloudflare/circl` bumped to `v1.6.3`
- [CVE-2026-35469](https://github.com/advisories/GHSA-pc3f-x583-g7j2) — `github.com/moby/spdystream` bumped to `v0.5.1`
- [CVE-2026-32952](https://github.com/advisories/GHSA-pjcq-xvwq-hhpj) — `github.com/Azure/go-ntlmssp` bumped to `v0.1.1`
- x/crypto→v0.53.0, x/net→v0.56.0, x/sys→v0.46.0, x/text→v0.39.0, `go.mongodb.org/mongo-driver`→v1.17.7: x/crypto CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597; x/net CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600; x/sys CVE-2026-39824; x/text CVE-2026-56852; mongo-driver CVE-2026-2303.

The grafana `stdlib` CVEs (CVE-2026-39822, CVE-2026-42505) are not fixable by a dependency bump —
the Go toolchain comes from the shared `builder/golang-bookworm` base image; they are handled via
`known_vulnerabilities.vex`.
