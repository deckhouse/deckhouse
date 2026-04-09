## Patches

### 001-go-mod.patch

Update dependencies

### 002-pma-log-flooding.patch

Log CPU metrics fetch failures as info instead of error for pods created within the monitoring window, reducing noise without lowering overall log verbosity.

### 003-fix-cves.patch

Fix CVE-2026-33186 and CVE-2026-24051.
```sh
go get google.golang.org/grpc@v1.79.3
go get go.opentelemetry.io/otel/sdk@v1.40.0
go mod tidy
```
