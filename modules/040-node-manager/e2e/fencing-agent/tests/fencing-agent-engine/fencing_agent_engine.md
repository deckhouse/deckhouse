# Fencing Agent on Deckhouse Engine

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test for fencing on a NodeGroup with `systemType: Immutable` (Deckhouse Engine).

On an Engine node neither bashible nor the `enable-watchdog-for-nodegroup-<ng>.sh` NodeGroupConfiguration runs, so the pieces a Mutable node gets from them come from elsewhere:

| What | Mutable node | Engine node |
| ---- | ------------ | ----------- |
| `softdog` | `modprobe softdog soft_panic=1` | built into the kernel, `softdog.soft_panic=1` on the UKI cmdline |
| `/dev/watchdog` owned by uid 64535, mode 0660 | udev rule written by bashible | udev rule shipped in the rootfs |
| Watchdog timeout | fencing-agent, `WDIOC_SETTIMEOUT` with the FencingSLAProfile timeout | the same — a built-in softdog cannot take `soft_margin` any other way |
| `kernel.panic=0` for a fenced group | `041_configure_sysctl_tuner.sh` | node-controller publishes it in NodeConfig `spec.kernel.sysctl`, nodelet applies it |

The test checks every row on the live nodes, then checks that the planned paths (maintenance, a restart of every agent at once, disabling fencing) disarm the watchdog with Magic Close and reset no node.

## Prerequisites

- A NodeGroup with `systemType: Immutable` and at least one Ready node, **without** `spec.fencing`.
- The nodes run a Deckhouse Engine release with softdog built in (Engine `v0.1.16`, kernel `v6.12.109-lvc30-1-signed` or newer). The kernel and the cmdline travel in the UKI, so a node installed earlier gets them only with a reinstall or a boot partition update; the baseline step fails with an explicit message on such a node.
- The probe image (`busybox:1.36` by default) is pullable from the nodes.
- Chainsaw CLI, `kubectl` and `jq`. See `../../README.md`.

## Parameters

| Environment variable | Default | Meaning |
| -------------------- | ------- | ------- |
| `FENCING_ENGINE_NODE_GROUP` | `worker` | NodeGroup under test |
| `FENCING_ENGINE_PROBE_IMAGE` | `busybox:1.36` | Image of the probe DaemonSet (needs `sh`, `cat`, `sleep`) |

## Test Steps

| Step | Name | Description |
| ---- | ---- | ----------- |
| 1 | `preflight` | The group is `Immutable`, has Ready nodes and no `spec.fencing`; records every node's `bootID` and the number of other fenced groups |
| 2 | `deploy-probe` | Deploys an unprivileged probe DaemonSet on the group (restricted Pod Security Standard) |
| 3 | `assert-baseline` | NodeConfig and the node have `kernel.panic=10`; `watchdog0` is `Software Watchdog`, `nowayout=0`, `inactive`; `/proc/cmdline` has `softdog.soft_panic=1` |
| 4 | `enable-fencing` | Sets `spec.fencing.mode: Critical`; cleanup always removes it and the maintenance annotation |
| 5 | `wait-for-agents` | `fencing-agent-<group>` is ready (the agent opened `/dev/watchdog` as uid 64535); every node has the condition `Fencing` `True`/`Armed`, the annotations `fencing-profile=critical` and `fencing-cp-source` (`proxy` or `self`), and no `fencing-enabled` or `fencing-mode` label of fencing 1.0 |
| 6 | `assert-fenced` | NodeConfig and the node have `kernel.panic=0`; `watchdog0` is `active` with the timeout of the group's FencingSLAProfile; a `WatchdogArmed` Event per node |
| 7 | `assert-maintenance` | `node-manager.deckhouse.io/fencing-disable` on one node disarms the watchdog (`inactive`, `WatchdogDisarmed` Event); the condition `Fencing` is `False`/`Maintenance`; after the profile timeout the node is still on the same boot; removing the annotation re-arms it and the condition is `True`/`Armed` |
| 8 | `restart-agents` | Deletes every agent pod of the group at once; the new agents re-arm and no node is reset |
| 9 | `disable-fencing` | Removes `spec.fencing`; the condition `Fencing` is `False`/`GroupDisabled` and the annotation `fencing-profile` is `disabled` while another group is fenced, or the DaemonSet, the condition and the annotations are gone when no group is |
| 10 | `assert-unfenced` | NodeConfig and the node are back to `kernel.panic=10`, the watchdog is `inactive`, no node was reset |

"No node was reset" means every node recorded in step 1 is `Ready` with the same `status.nodeInfo.bootID`.

## Files

| File | Purpose |
| ---- | ------- |
| `chainsaw-test.yaml` | Chainsaw test definition |
| `manifests/probe-daemonset.yaml` | Unprivileged probe reading `/proc` and `/sys/class/watchdog` |
| `manifests/nodegroup-fencing.yaml` | Enables fencing on the group in the `Critical` mode |
| `asserts/*.yaml` | NodeGroup and DaemonSet assertions |
| `scripts/lib.sh` | Helpers shared by the step scripts |
| `scripts/*.sh` | One script per node-side step |

## Running

```bash
export FENCING_ENGINE_NODE_GROUP=engine-worker

# From the test directory
task run

# From the fencing-agent root
task fencing-agent-engine:run
```

Plan ~15 minutes: NodeConfig rollouts, the agent rollout and three waits of the profile timeout.

## Troubleshooting

### `assert-baseline` fails on `softdog.soft_panic=1` or the watchdog identity

The node boots an Engine UKI without softdog. Reinstall the node or update its boot partition.

### `wait-for-agents` fails

```bash
kubectl -n d8-cloud-instance-manager logs -l app=fencing-agent,node-group-name=$FENCING_ENGINE_NODE_GROUP
kubectl get events -A --field-selector reason=WatchdogRefused
```

`WatchdogRefused` names the reason: no `WDIOF_SETTIMEOUT`, a timeout the driver rejected, or `nowayout`.

### `assert-fenced` stays on `kernel.panic=10`

Check what node-controller published and what the node applied:

```bash
kubectl get nodeconfig <node> -o jsonpath='{.spec.kernel.sysctl}'
kubectl get nodeconfig <node> -o jsonpath='{.status}'
```

## Safety

The test changes `spec.fencing` of the group under test and annotates one of its nodes; the step 4 cleanup reverts both, even on failure. If the test process is killed:

```bash
kubectl annotate nodes -l node.deckhouse.io/group=$FENCING_ENGINE_NODE_GROUP node-manager.deckhouse.io/fencing-disable-
kubectl patch nodegroup $FENCING_ENGINE_NODE_GROUP --type=merge -p '{"spec":{"fencing":null}}'
```

With fencing on, a node whose agent stops feeding without a disarm panics and stays down (`kernel.panic=0`). The test never provokes that on purpose.
