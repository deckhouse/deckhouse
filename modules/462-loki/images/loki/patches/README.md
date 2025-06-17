# Patches

## 001-go-mod.patch

    Fix CVEs in crypto/net packages.
    ```sh
    go get golang.org/x/crypto v0.31.0
    go get golang.org/x/net v0.33.0
    go mod tidy
    ```

## 002-Allow-delete-logs.patch

TODO

## 003-Force-expiration.patch

TODO

## 004-Force-expiration-index-sort.patch

Fix incorrect indices sort function used in disk-based retention.
Use `sortTablesByRangeOldestFirst` sort function to mark the oldest chunks as expired.
