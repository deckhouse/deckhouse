## Patches

### 001-go-mod.patch

Update dependencies

### 002-pma-log-flooding.patch

Log CPU metrics fetch failures as info instead of error for pods created within the monitoring window, reducing noise without lowering overall log verbosity.

### 003-cve-go-mod.patch

Bump `google.golang.org/grpc` to v1.79.3 (fixes CVE-2026-33186) and `go.opentelemetry.io/otel/sdk` to v1.43.0 (fixes CVE-2026-24051 and CVE-2026-39883). Required `go.sum`/transitive updates and a `go 1.25.0` directive bump are included.

Also bump `golang.org/x/crypto`→v0.53.0, `golang.org/x/net`→v0.56.0, `golang.org/x/sys`→v0.46.0 and
`golang.org/x/text`→v0.39.0 to fix CVEs reported by Trivy against the embedded Go dependencies:
- x/crypto: CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597
- x/net: CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600
- x/sys: CVE-2026-39824
- x/text: CVE-2026-56852
