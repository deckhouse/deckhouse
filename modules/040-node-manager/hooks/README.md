# node-manager hooks

Go hooks of the module, grouped by the team that owns the feature a hook serves. Each folder is a Go package with its own test suite and a README that says what every hook does and why.

| Folder | Domain | Owners |
|---|---|---|
| `core` | NodeGroup lifecycle, bashible, CAPI and MCM plumbing, certificates of the module's own components | node-manager team, the module default in CODEOWNERS |
| `scheduling` | cluster-autoscaler, standby nodes, NodeGroup priorities, fencing | owners of `templates/cluster-autoscaler` and `templates/fencing-agent` |
| `cloud` | cloud providers | owners of `030-cloud-provider-*` |
| `caps` | CAPS, the Cluster API provider for static nodes | owners of `templates/caps-controller-manager` |
| `kubernetes` | containerd and kubelet | owners of the containerd and kubelet images |
| `gpu` | NVIDIA GPU support | owners of `templates/nvidia-gpu` |
| `upmeter` | bridge to the upmeter module | owners of the observability modules |

`internal/` holds shared types and helpers (CAPI and MCM API types, NodeGroup versions, kubeconfig generation), `pkg/schema` the values schema helpers.

A new hook goes into the folder of the team that will answer for the feature. If no folder fits, the hook is probably not a node-manager hook.

The hook name that addon-operator reports in metrics and in `deckhouse-controller module snapshots` includes the folder, for example `040-node-manager/hooks/core/get_crds.go`.
