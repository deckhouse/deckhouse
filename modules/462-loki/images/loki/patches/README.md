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

## 004-cve-grpc-jsonparser.patch

Fix CVE-2026-33186 (`google.golang.org/grpc` < v1.79.3) and CVE-2026-32285
(`github.com/buger/jsonparser` < v1.1.2).

```sh
go get google.golang.org/grpc@v1.79.3
go get github.com/buger/jsonparser@v1.1.2
# Minimum google.golang.org/api version that uses keyed-field initialization
# of grpcgoogle.DefaultCredentialsOptions (vendored google API code does not
# compile against grpc >= 1.64 otherwise).
go get google.golang.org/api@v0.155.0
go mod tidy -e
```

The patch also adds a small `healthCheckWithList` wrapper in `pkg/loki/loki.go`
because dskit's `grpcutil.HealthCheck` (pinned at the loki v2.9.15 version) does
not implement the `List` RPC that grpc >= 1.64 added to the
`grpc_health_v1.HealthServer` interface. Bumping dskit to a version that
implements `List` would cascade into incompatible memberlist/prometheus changes.
