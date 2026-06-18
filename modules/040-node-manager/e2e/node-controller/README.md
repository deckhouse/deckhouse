# node-controller e2e

Real-cluster end-to-end tests for the `node-manager` **node-controller**, grouped by user
scenario. Each test drives the platform the way a user would (through a `NodeGroup`) and checks
the user-visible outcome on a **live DVP/CAPI cluster**.

| Test | Scenario |
| --- | --- |
| `scale-cloud-node` | scale a CloudEphemeral node up (order one real node) and back down (give it back) |

## scale-cloud-node

A user creates a `CloudEphemeral` `NodeGroup`, the platform orders one real node, and on teardown
gives it back. The provisioning chain

```text
NodeGroup → MachineDeployment → MachineSet → Machine → Node   (+ the derived Instance)
```

is owned by the **set-replicas / CAPI / cloud-provider** controllers — not by any single
node-manager controller — so this is an integration test of the chain, not a unit/controller test.

> **Not the Instance controller test.** The node-controller's *Instance* controller is verified
> by its **envtest** suite at
> `images/node-controller/src/internal/controller/instance/e2e_*_test.go` (status mapping, bashible
> aggregation/heartbeat, self-heal, GC, deletion, SSA field ownership, negative branches — ≥90%
> statement coverage, MCM excluded). That is where the controller's logic lives. Here the Instance
> is only checked **as a by-product** of a node being ordered, because on a real cluster it
> reflects the REAL machine + bashible state — the one thing envtest cannot fake.

## Suite layout

```text
e2e/node-controller/
├── chainsaw-config.yaml
├── Taskfile.yaml
└── tests/scale-cloud-node/
    ├── chainsaw-test.yaml
    ├── manifests/nodegroup.yaml             # the CloudEphemeral NodeGroup to provision
    └── asserts/                             # declarative assert/error trees
```

`scale-cloud-node` walks the chain: apply a NodeGroup → it reports a ready node with internally
consistent status counters → CAPI's MachineDeployment provisions a Machine (phase Running,
Ready/InfrastructureReady/NodeReady, a populated nodeRef) → the node joins Ready → (by-product) the
derived Instance reflects the real machine + bashible (`machineRef`, `phase`/`machineStatus`/
`bashibleStatus`, `MachineReady`/`BashibleReady`, the `bashible:` message) → delete the NodeGroup →
the Machine, Node and Instance are removed.

It is **fully declarative**: every check is a chainsaw `assert`/`error` tree that polls until the
state matches, so the test is flap-proof, reads top to bottom and contains **no embedded scripts**
(only a one-line `until` in cleanup that waits for the node to drain).

CAPI `Machine`/`MachineDeployment` objects are **not readable by the default cluster identity**
(only the CAPI controllers may read them), so the suite is run **impersonating the deckhouse
super-admin** — `chainsaw test --kube-as=system:sudouser`, wired into the test's `Taskfile.yml`
(override `KUBE_AS` if your kubeconfig already has full CAPI access). The Machine is located **via
its MachineDeployment** — both carry the `node-group=<NG name>` label; the cluster-scoped,
unlabeled Instance is matched as a collection by a `name` predicate.

On any step failure the suite-wide `error.catch` in `chainsaw-config.yaml` collects the
**`node-controller` and `capi-controller-manager` logs** (the `kube-rbac-proxy` sidecar is skipped
by naming the real container) plus the `d8-cloud-instance-manager` events — so both the controllers'
reasoning and the kube-level signals are captured without per-step `catch` handlers in the test.

## Running

```bash
# from e2e/node-controller/
task scale-cloud-node:dry-run   # chainsaw lint only, no cluster needed
task scale-cloud-node:run       # provisions a real VM (~5-15 min)
```

```bash
# the Instance controller itself (envtest, from images/node-controller/src/)
make test                          # downloads envtest assets and runs unit + envtest
```

Chainsaw run environment needs `chainsaw` and `kubectl` (pointed at the cluster). The identity used
must be allowed to impersonate `system:sudouser` (deckhouse super-admin) so the Machine/Machine
Deployment asserts can read CAPI objects. JUnit reports are written to the suite's `reports/`.

## Safety

`scale-cloud-node` creates an isolated `NodeGroup` referencing the existing `worker`
`DVPInstanceClass` and provisions a real VM; every `apply` step has a matching `cleanup: delete`,
and the final cleanup waits until all `e2e-ephemeral-node` nodes are removed.
