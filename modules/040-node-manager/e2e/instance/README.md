# instance controller e2e tests

End-to-end tests for the **Instance controller** of the `node-manager` module.

The controller's behaviour is coupled to the machine and bashible:

- **CAPI machine** — the `cluster.x-k8s.io` `Machine` that backs a `CloudEphemeral` Instance;
- **bashible** — the on-node agent that owns the `BashibleReady` / `WaitingApproval` /
  `WaitingDisruptionApproval` conditions via Server-Side Apply;
- **Node** — the source for `Static` / `CloudPermanent` Instances.

Testing is split into two layers (see `TESTPLAN.md` for the full case-by-case mapping):

| Layer | Where | What it covers |
| --- | --- | --- |
| **envtest + ginkgo** | `images/node-controller/src/internal/controller/instance/e2e_*_test.go` | every status mapping, bashible aggregation/heartbeat, self-heal, GC, deletion, negative branches — driven through a real kube-apiserver with the controller running in a manager. ≥90% statement coverage of the controller (MCM excluded). |
| **chainsaw** (this dir) | `tests/instance-lifecycle-scale/` | the real machine + real bashible lifecycle on a **live** DVP/CAPI cluster. |

**Why two layers.** Deckhouse RBAC makes `Instance` and `*InstanceClass` objects **not
user-writable** (only the controller may create/patch/delete them). Anything that needs a
mutated `Instance` status/spec — self-heal, heartbeat staleness, conflicts, the negative
node branches — can only be tested in envtest, where the test owns the API. The chainsaw
suite drives the controller *indirectly* through a `NodeGroup` (which is user-writable).

## Chainsaw suite layout

```text
e2e/instance/
├── chainsaw-config.yaml
├── Taskfile.yaml
├── TESTPLAN.md                              # full machine + bashible coverage mapping
└── tests/instance-lifecycle-scale/
    ├── chainsaw-test.yaml
    ├── manifests/nodegroup.yaml             # the CloudEphemeral NodeGroup to provision
    └── asserts/                             # declarative assert/error trees
```

`chainsaw-test.yaml` is declarative end to end: chainsaw asserts poll until the state
matches, so they are flap-proof and read top to bottom. The contract on the converged
Instance (self-healed `machineRef`/`nodeRef`, finalizer, `phase`/`machineStatus`/
`bashibleStatus`, `MachineReady`/`BashibleReady` conditions, the `bashible:` message and the
real Server-Side-Apply field ownership between the controller and bashible) is one
declarative `assert` resource tree. Two small recorder steps print the Instance
phase/machineStatus/bashibleStatus **timeline** as the VM is ordered and again as it is torn
down, so the whole lifecycle is visible in the test output. The only script besides those is
a one-line resolver for the dynamic Instance name.

## Running

```bash
# from e2e/instance/
task instance-lifecycle-scale:dry-run   # chainsaw lint only, no cluster needed
task instance-lifecycle-scale:run       # provisions a real VM (~5-15 min)
```

```bash
# the envtest layer (from images/node-controller/src/)
make test                               # downloads envtest assets and runs unit + envtest
```

Chainsaw run environment needs `chainsaw`, `kubectl` (pointed at the cluster) and `jq`.
JUnit reports are written to the suite's `reports/`.

## Safety

`instance-lifecycle-scale` creates an isolated `NodeGroup` referencing the existing `worker`
`DVPInstanceClass` and provisions a real VM; every `apply` step has a matching `cleanup:
delete`, and the final cleanup waits until all `e2e-instance` nodes are removed.
