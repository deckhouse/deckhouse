---
title: "Automatic backup API guide"
permalink: en/stronghold/documentation/user/auto-snapshot.html
lang: en
description: "Administrator's guide to working with the Deckhouse Stronghold automatic snapshot API."
---

Deckhouse Stronghold lets you configure a schedule for automatic secret storage backups.
Since Stronghold stores data on disk in encrypted form, the backup also contains only encrypted data.
To access the data, you need to restore the backup in a Stronghold cluster and perform the unsealing procedure.

Backups can be stored either on a local disk in the selected directory or in an S3-compatible storage.

You can manage backup settings and check their status via the API, the Stronghold CLI, and the web UI.

## Creating or updating an automatic backup configuration

| Method | Path |
|--------|------|
| POST   | `/sys/storage/raft/snapshot-auto/config/:name` |

Sudo privileges are required to use this API method.

### Parameter description

<div class="table__styling--container"></div>

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | String | Yes | — | Name of the configuration to create or update. |
| `interval` | Integer or string | Yes | — | Interval between backups. Can be specified in seconds or in Go duration format (for example, `24h`). |
| `retain` | Integer | No | `3` | Number of backups to keep. When this number is exceeded, the oldest backups are deleted. |
| `path_prefix` | Immutable string | Yes | — | If `storage_type` is set to local storage, this specifies the backup directory. If set to cloud storage, this specifies the bucket prefix (a leading `/` is ignored, subsequent `/` are optional). |
| `file_prefix` | Immutable string | No | `stronghold-snapshot` | File or object name prefix for the backup within the directory or bucket specified in `path_prefix`. |
| `storage_type` | Immutable string | Yes | — | Backup storage type: `local` or `aws-s3` (cloud). The parameters below depend on the selected storage type. |

#### Additional parameters for local storage

<div class="table__styling--container"></div>

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `local_max_space` | Integer | No | `0` | Maximum available space (in bytes) for backups with the given `file_prefix` in the `path_prefix` directory. If available space is insufficient, backup creation fails. A value of `0` disables disk space checks. |

#### Additional parameters for cloud storage

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `aws_s3_bucket` | String | Yes | — | Name of the S3 bucket for storing backups. |
| `aws_s3_region` | String | No | — | Region of the S3 bucket. |
| `aws_access_key_id` | String | No | — | Key ID for accessing the S3 bucket. |
| `aws_secret_access_key` | String | No | — | Secret key for accessing the S3 bucket. |
| `aws_s3_endpoint` | String | No | — | S3 service endpoint. |
| `aws_s3_disable_tls` | Boolean | No | — | Disables TLS for the S3 endpoint. Used only for testing, usually together with `aws_s3_endpoint`. |
| `aws_s3_ca_certificate` | String | No | — | CA certificate for the S3 endpoint in PEM format. |

### Request examples

#### Creating a configuration

All required fields must be specified.

```shell
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "5m",
    "path_prefix":       "backups",
    "file_prefix":       "main_stronghold",
    "retain":            "4",
    "storage_type":      "aws-s3",
    "aws_s3_bucket":         "my_bucket",
    "aws_s3_endpoint":       "minio.domain.ru",
    "aws_access_key_id":     "<ACCESS_KEY>",
    "aws_secret_access_key": "<SECRET_ACCESS_KEY>"
}
EOF
```

Example response:

```console
Key    Value
---    -----
msg    successfully created config
```

#### Updating a configuration

Not all fields need to be provided. Existing fields remain unchanged if omitted.

```shell
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "3m",
    "retain":            "10",
    "aws_access_key_id":     "<ACCESS_KEY>",
    "aws_secret_access_key": "<SECRET_ACCESS_KEY>"
}
EOF
```

Example response:

```console
Key    Value
---    -----
msg    successfully updated config
```

## Viewing the list of existing configurations

| Method | Path |
|--------|------|
| LIST   | `/sys/storage/raft/snapshot-auto/config` |

Returns a list of all existing automatic backup configurations.

### Request example

```shell
d8 stronghold list sys/storage/raft/snapshot-auto/config
```

Example response:

```console
Keys
----
s3every5min
localEvery3min
```

## Obtaining configuration parameters

| Method | Path |
|--------|------|
|  GET   | `/sys/storage/raft/snapshot-auto/config/:name` |

Returns the parameter values of the specified configuration.

### Request example

```shell
d8 stronghold read sys/storage/raft/snapshot-auto/config/s3every5min
```

Example response:

```console
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

## Deleting a configuration

| Method | Path |
|--------|------|
| DELETE | `/sys/storage/raft/snapshot-auto/config/:name` |

Deletes the specified configuration and returns information about the last created backup.

### Request example

```shell
d8 stronghold delete sys/storage/raft/snapshot-auto/config/s3every5min
```

Example response:

```console
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

## Getting backup status

| Method | Path |
|--------|------|
|  GET   | `/sys/storage/raft/snapshot-auto/status/:name` |

Returns information about the current status of the specified backup.

### Request example

```shell
d8 stronghold read sys/storage/raft/snapshot-auto/status/s3every5min
```

Example response:

```console
Key    Value
---    -----
msg    successfully deleted config
```
