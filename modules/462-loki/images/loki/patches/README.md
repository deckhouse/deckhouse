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
