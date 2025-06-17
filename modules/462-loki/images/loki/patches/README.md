# Patches

## 001-go-mod.patch

    Fix CVEs in crypto/net packages.
    ```sh
    go: upgraded golang.org/x/crypto v0.24.0 => v0.31.0
    go: upgraded golang.org/x/net v0.26.0 => v0.33.0
    ```

## 002-Allow-delete-logs.patch

TODO

## 003-Force-expiration.patch

TODO

## 004-Force-expiration-index-sort.patch

Fix incorrect indices sort function used in disk-based retention.  

Use `sortTablesByRangeOldestFirst` sort function to mark the oldest chunks as expired.  

Add new metrics:
- `force_expiration_hook_index_range`
- `force_expiration_hook_first_expired_chunk_timestamp_seconds`

to monitor the sorting results along with existing `force_expiration_hook_last_expired_chunk_timestamp_seconds`
