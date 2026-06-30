# rpp-get

CLI tool for downloading and installing packages from a [Registry Packages Proxy](../../README.md) (RPP).

## Design constraints

The binary runs on cluster nodes during bootstrapping, before the node is fully operational. This drives two hard constraints:

- **Minimal dependencies.** The only non-stdlib dependency is `k8s.io/client-go`, required for kubeconfig parsing and TLS client construction. New dependencies must have a strong justification.
- **No assumed environment.** The tool does not rely on a container runtime, system package manager, or any network service other than RPP and kube-apiserver.

## Modes

| Mode | Description |
|------|-------------|
| `fetch` | Download package archives to a temporary directory |
| `install` | Download and install packages |
| `uninstall` | Remove installed packages |

## Structure

```
rpp-get/
├── src/
│   ├── main.go          # Entry point and orchestration
│   ├── config.go        # Argument parsing, endpoint and token resolution
│   ├── constants.go     # Main package constants
│   ├── utils.go         # parseEndpoints, waitRetry
│   ├── kube/
│   │   └── client.go    # kube-apiserver HTTP client (GetRPPEndpoints, GetRPPToken)
│   └── rpp/
│       ├── client.go    # Core logic: fetch / install / uninstall
│       ├── http.go      # RPP HTTP client
│       ├── result.go    # Result file writer
│       ├── archive.go   # tar.gz extraction
│       ├── utils.go     # Internal helpers
│       └── constants.go # rpp package constants
└── scripts/
```

## Configuration sources (evaluated in order)

1. CLI flags (`--rpp-endpoints`, `--rpp-token`)
2. Environment variables (`PACKAGES_PROXY_ADDRESSES`, `PACKAGES_PROXY_TOKEN`)
3. kube-apiserver via kubelet kubeconfig (`/etc/kubernetes/kubelet.conf`)
4. kube-apiserver via bootstrap token (`/var/lib/bashible/bootstrap-token`) + `--kube-apiserver-endpoints`

If neither endpoints nor token are supplied explicitly (flags or env), the tool queries kube-apiserver to obtain them. It first attempts to use the kubelet kubeconfig (`/etc/kubernetes/kubelet.conf`). If that file is absent, it falls back to the bootstrap token (`/var/lib/bashible/bootstrap-token`); in that case the kube-apiserver address must be provided via `--kube-apiserver-endpoints`, as there is no other source for it at that stage.

## Direct registry mode

By default `rpp-get` downloads package archives from RPP. With `--registry-direct` it instead pulls them straight from the container registry over the OCI Distribution v2 protocol, bypassing RPP entirely. This removes the dependency on the rpp server (and, during bootstrap, on the SSH tunnel to the dhctl machine).

Direct mode requires the registry connection parameters. It does not query kube-apiserver and does not resolve RPP endpoints.

| Flag                  | Env                | Description                              |
|-----------------------|--------------------|------------------------------------------|
| `--registry-direct`   | `REGISTRY_DIRECT`  | enable direct mode                       |
| `--registry-repo`     | `REGISTRY_REPO`    | full registry repository (`host/path`)   |
| `--registry-auth`     | `REGISTRY_AUTH`    | `base64(user:password)` registry auth    |
| `--registry-ca-file`  | `REGISTRY_CA_FILE` | path to a PEM CA bundle (optional)       |
| `--registry-scheme`   | `REGISTRY_SCHEME`  | `https` (default) or `http`              |

`rpp-get` fetches the image manifest by digest, selects its last layer, and streams that layer's blob — the same gzipped tar archive RPP would return. Image signatures are not verified in direct mode; integrity is guaranteed by the manifest digest.
