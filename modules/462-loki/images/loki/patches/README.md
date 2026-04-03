# Patches

## 001-go-mod.patch

    Fix CVEs in crypto/net packages.
    ```sh
    go get golang.org/x/crypto v0.31.0
    go get golang.org/x/net v0.33.0
    go mod tidy
    ```

## 002-Allow-delete-logs.patch

Enable/disable `/loki/api/v1/delete` endpoints by setting `ALLOW_DELETE_LOGS` env value to true/false.

## 003-Force-expiration.patch

Automatically delete old logs by setting `force_expiration_threshold` higher than 0.

## 004-fix-cves.patch

Fix CVE-2026-33186 and CVE-2026-32285.
```sh
go get google.golang.org/grpc@v1.79.3
go get github.com/buger/jsonparser@v1.1.2
go get google.golang.org/api@v0.215.0
go mod tidy
```
Also adds a health check adapter in `pkg/loki/health_check_adapter.go` to satisfy
the updated `grpc_health_v1.HealthServer` interface (new `List` method) without
requiring a `grafana/dskit` upgrade.
