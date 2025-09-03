---
title: "Deckhouse Stronghold Administrator's Guide to the Automatic Snapshot API"
permalink: en/stronghold/documentation/user/auto-snapshot.html
lang: en
description: "Administrator's guide to working with the Deckhouse Stronghold automatic snapshot API."
---

Deckhouse Stronghold supports creating schedules for performing automatic backups of the internal storage.
Since Stronghold stores data on disk in encrypted form, the backup will also contain only encrypted data.
To access the data, you will need to restore the backup to the Stronghold cluster and perform the storage unseal procedure.

Backups can be saved to a local disk in a selected folder or to S3-compatible storage.

Backup settings can be managed and their status viewed via API, CLI, and UI.

## Creating/updating automatic snapshot configuration

| Method | Path |
|--------|------|
| POST   | /sys/storage/raft/snapshot-auto/config/:name |

Sudo privileges are required to interact with this API method.

### Parameters

- **name (string: <required>)** – Name of the configuration to modify.

- **interval (integer or string: <required>)** - Time between snapshots. This can be either an integer number of seconds, or a Go duration format string (e.g. 24h)

- **retain (integer: 3)** - How many snapshots are to be kept; when writing a snapshot, if there are more snapshots already stored than this number, the oldest ones will be deleted.

- **path_prefix (immutable string: <required>)** - For `storage_type=local`, the directory to write the snapshots in. For s3 storage type, the bucket prefix to use. The trailing `/` is optional.

- **file_prefix (immutable string: "stronghold-snapshot")** - Within the directory or bucket prefix given by `path_prefix`, the file or object name of snapshot files will start with this string.

- **storage_type (immutable string: <required>)** - One of `local` or `aws-s3`. The remaining parameters described below are specific to the selected `storage_type` and prefixed accordingly.

#### storage_type = "local"

- **local_max_space (integer: 0)** - For `storage_type=local`, the maximum space, in bytes, to use for all snapshots with the given `file_prefix` in the `path_prefix` directory. Snapshot attempts will fail if there is not enough space left in this allowance. A value of 0 (default) disables space checking.

#### storage_type = "aws-s3"

- **aws_s3_bucket (string: <required>)** - S3 bucket to write snapshots to.
- **aws_s3_region (string)** - AWS region bucket is in.
- **aws_access_key_id (string)** - AWS access key ID.
- **aws_secret_access_key (string)** - AWS secret access key.
- **aws_s3_endpoint (string)** - AWS endpoint. This is typically only set when using a non-AWS S3 implementation like Minio.
- **aws_s3_disable_tls (boolean)** - Disable TLS for the S3 endpoint. This should only be used for testing purposes, typically in conjunction with `aws_s3_endpoint`.
- **aws_s3_ca_certificate (string)** - Certificate authority certificate for the `aws_s3_endpoint` in PEM format.

### Example

#### Creation

All required fields are specified

```sh
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "5m",
    "path_prefix":       "backups",
    "file_prefix":       "main_stronghold",
    "retain":            "4",
    "storage_type":      "aws-s3",
    "aws_s3_bucket":         "my_bucket",
    "aws_s3_endpoint":       "minio.domain.ru",
    "aws_access_key_id":     "oWdPcQ50zTuMjJI",
    "aws_secret_access_key": "4NzZjboafWyfNTe7aUVgLUdrMurHjty43iUXHFBw"
}
EOF
```

Response:

```
Key    Value
---    -----
msg    successfully created config
```

#### Update

You can specify not all fields; existing fields will not be changed

```sh
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "3m",
    "retain":            "10",
    "aws_access_key_id":     "vnR9Rfp0toPPgK3",
    "aws_secret_access_key": "FuloGN1RZCtwINCLJtwHXTQ50zCL7s"
}
EOF
```

Response:

```
Key    Value
---    -----
msg    successfully updated config
```

## List existing automatic snapshot configurations

| Method | Path |
|--------|------|
| LIST   | /sys/storage/raft/snapshot-auto/config |

Used to get a list of names of all existing automatic snapshots
### Example

`d8 stronghold list sys/storage/raft/snapshot-auto/config`

Response:

```
Keys
----
s3every5min
localEvery3min
```

## Obtaining automatic snapshot configuration parameters

| Method | Path |
|--------|------|
|  GET   | /sys/storage/raft/snapshot-auto/config/:name |

### Example

`d8 stronghold read sys/storage/raft/snapshot-auto/config/s3every5min`

Response:

```
Key                     Value
---                     -----
interval                300
path_prefix             backups
file_prefix             main_stronghold
retain                  4
storage_type            aws-s3
aws_s3_bucket           my_bucket
aws_s3_disable_tls      false
aws_s3_endpoint         minio.domain.ru
aws_s3_region           n/a
aws_s3_ca_certificate   n/a
```

## Deleting the automatic snapshot configuration

| Method | Path |
|--------|------|
| DELETE | /sys/storage/raft/snapshot-auto/config/:name |

### Example

`d8 stronghold delete sys/storage/raft/snapshot-auto/config/s3every5min`

Response:

```
Key                    Value
---                    -----
consecutive_errors     0
last_snapshot_end      2025-01-31T15:24:14Z
last_snapshot_error    n/a
last_snapshot_start    2025-01-31T15:24:12Z
last_snapshot_url      https://minio.domain.ru/my_bucket/backups/main_stronghold_2025-01-31T15:24:12Z
next_snapshot_start    2025-01-31T15:29:12Z
snapshot_start         2025-01-31T15:24:12Z
snapshot_url           https://minio.domain.ru/my_bucket/backups/main_stronghold_2025-01-31T15:24:12Z
```

## Getting the status of the automatic snapshot

| Method | Path |
|--------|------|
|  GET   | /sys/storage/raft/snapshot-auto/status/:name |

### Example

`d8 stronghold read sys/storage/raft/snapshot-auto/status/s3every5min`

Response:

```
Key    Value
---    -----
msg    successfully deleted config
```
