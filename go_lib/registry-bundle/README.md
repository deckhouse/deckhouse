# Registry

A container registry server that implements the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec) and the [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/). It serves images from **Deckhouse chunked bundle archives** on disk (directory scan for `*.tar` and `*.tar.*.chunk` style layouts).

## Build

```bash
go build -o bundle-registry ./cmd/bundle-registry
```

## Usage

The binary exposes a Cobra CLI. Run the server with the **`serve`** subcommand:

```bash
./bundle-registry serve <bundle-path> [flags]
```

**`bundle-path`** (required): directory to scan for bundle archives (chunked `.tar.*.chunk` or whole `.tar`).

`serve` flags:

| Flag                 | Short | Default             | Description                                                                    |
| -------------------- | ----- | ------------------- | ------------------------------------------------------------------------------ |
| `--address`          | `-a`  | `localhost:5001`    | TCP listen address (`host:port`)                                               |
| `--root-repo`        | `-r`  | `system/deckhouse`  | Virtual registry path prefix for the merged repo                               |
| `--tls-cert`         |       | *(none)*            | TLS certificate path (use with `--tls-key`)                                    |
| `--tls-key`          |       | *(none)*            | TLS private key path (use with `--tls-cert`)                                   |

### Examples

Serve from a directory of chunked bundles:

```bash
./bundle-registry serve /path/to/bundles
```

With listen address:

```bash
./bundle-registry serve ./bundle-chunks/ --address 0.0.0.0:5001
```

Virtual repo path (clients pull under `/v2/<root-repo>/...`):

```bash
./bundle-registry serve ./bundles/ --root-repo system/deckhouse
```

HTTPS (paths must be readable by the process):

```bash
./bundle-registry serve ./bundles/ --tls-cert server.crt --tls-key server.key
```
