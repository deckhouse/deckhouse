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
