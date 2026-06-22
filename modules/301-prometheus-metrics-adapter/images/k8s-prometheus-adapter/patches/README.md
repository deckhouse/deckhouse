## Patches

### 001-go-mod.patch

Update dependencies

### 002-pma-log-flooding.patch

Log CPU metrics fetch failures as info instead of error for pods created within the monitoring window, reducing noise without lowering overall log verbosity.

### 003-cve-go-mod.patch

Bump `google.golang.org/grpc` to v1.79.3 (fixes CVE-2026-33186) and `go.opentelemetry.io/otel/sdk` to v1.43.0 (fixes CVE-2026-24051 and CVE-2026-39883). Required `go.sum`/transitive updates and a `go 1.25.0` directive bump are included.
