# Registry Syncer

A tool for syncing all images and tags from a source container registry to a destination container registry. It discovers all repositories and tags in the source, compares manifests by digest, and copies only what has changed.

## Build

```bash
go build -o registry-syncer ./cmd/registry-syncer
```

## Usage

```bash
./registry-syncer <config-file>
```

**`config-file`** (required): path to a YAML configuration file describing source and destination registries.

## Configuration

The config file is YAML with two top-level keys: `source` and `destination`.

```yaml
source:
  address: registry.example.com
  user:                 # optional
    name: username
    password: secret
  ca: |                 # optional
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----

destination:
  address: dst-registry.example.com
  user:                 # optional
    name: username
    password: secret
  ca: |                 # optional
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

| Field                       | Required | Description                                          |
| --------------------------- | -------- | ---------------------------------------------------- |
| `source.address`            | yes      | Source registry hostname (and optional port)         |
| `source.user.name`          | no       | Username for source registry authentication          |
| `source.user.password`      | no       | Password for source registry authentication          |
| `source.ca`                 | no       | Custom CA certificate (PEM-encoded) for source       |
| `destination.address`       | yes      | Destination registry hostname (and optional port)    |
| `destination.user.name`     | no       | Username for destination registry authentication     |
| `destination.user.password` | no       | Password for destination registry authentication     |
| `destination.ca`            | no       | Custom CA certificate (PEM-encoded) for destination  |

### Examples

Sync between two unauthenticated registries:

```yaml
source:
  address: src-registry.internal:5000

destination:
  address: dst-registry.internal:5000
```

Sync with authentication and custom CA:

```yaml
source:
  address: src-registry.example.com
  user:
    name: puller
    password: pull-secret
  ca: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----

destination:
  address: dst-registry.example.com
  user:
    name: pusher
    password: push-secret
```

Run the sync:

```bash
./registry-syncer config.yaml
```

## Environment Variables

| Variable             | Default   | Description                                              |
| -------------------- | --------- | -------------------------------------------------------- |
| `LOG_LEVEL`          | `info`    | Log verbosity: `debug`, `info`, `warn`, `error`          |
| `SHOW_LOG_LEVEL`     | *(unset)* | Set to `true` to include log level in output             |
| `SHOW_LOG_TIMESTAMP` | *(unset)* | Set to `true` to include timestamps in logs              |

## How It Works

1. Catalogs all repositories from the source registry.
2. Lists all tags for each repository.
3. For each tag, fetches the manifest digest from both source and destination.
4. Skips tags where the destination digest matches the source (already in sync).
5. Pushes the manifest (and referenced blobs) to the destination for any tag that differs or is missing.
6. Failed tag syncs are retried up to 3 times with a 5-second interval before the process exits with an error.

Graceful shutdown is supported via `SIGINT` / `SIGTERM`. A second signal forces immediate exit.
