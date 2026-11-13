# Fencing Agent Deployment

## Summary

A [Kyverno Chainsaw](https://kyverno.github.io/chainsaw/) e2e test for the fencing 2.0 agents of the `node-manager` module.

The test creates its own NodeGroup with one cloud node and never patches an existing group. It switches the group through `Normal`, `Slow`, `Critical`, the deprecated `Watchdog` (works as `Slow`) and `Disabled`, and checks the state after each switch. No step stops a node: the boot ID of the node must stay the same.

The test uses only Chainsaw operations, with no shell scripts. Every change and every check is a step of its own, so the JUnit report shows which check failed. The test ends only when nothing of the test group is left in the cluster, so the next run does not find a node or a Machine of this run.

## Prerequisites

- A source NodeGroup with `nodeType: CloudEphemeral` (default name `worker`). The test copies only its instance class and first zone.
- Nodes support the `softdog` kernel module.
- Fencing-agent image built and available in the Deckhouse registry.
- The probe image (`busybox:1.36` by default) is pullable from the nodes.
- Chainsaw CLI 0.2.15 or later and `kubectl`. See `../../README.md`.

## Parameters

| Environment variable | Default | Meaning |
| -------------------- | ------- | ------- |
| `FENCING_TEST_NODE_GROUP` | `e2e-fencing` | Name of the NodeGroup the test creates and deletes. A group with this name that a previous run left is deleted first |
| `FENCING_SOURCE_NODE_GROUP` | `worker` | Existing `CloudEphemeral` group that gives the instance class and the zone. It is only read |
| `FENCING_INSTANCE_CLASS` | empty | Name of the instance class for the test group; replaces the class name of the source group |
| `FENCING_PROBE_IMAGE` | `busybox:1.36` | Image of the pods that read `kernel.panic` (needs `sh` and `cat`) |

The test group has no label of the source group, so an external controller that manages the source group does not touch it.

## Test Steps

Preparation:

| Steps | What they do |
| ----- | ------------ |
| `test-namespace-ready` | With the module `multitenancy-manager`, waits until the test namespace is in a Project of its own. Its cleanup deletes that Project (or the namespace, when the cluster has no Projects) and waits until the namespace is gone |
| `source-group-is-cloud-ephemeral` | The source group is `CloudEphemeral` |
| `delete-leftover-group` | Deletes the test group that a previous run left |
| `no-leftover-nodes`, `-machines`, `-instances`, `-daemonset`, `-watchdog-ngc` | One step each: no node, Machine, Instance, agent DaemonSet or watchdog NodeGroupConfiguration of the test group is left (waits up to 15 minutes) |
| `create-group` | Creates the test NodeGroup. Its cleanup runs when a step fails: deletes the group and waits until its nodes, Machines, DaemonSet and NodeGroupConfiguration are gone |
| `node-ready` | Waits up to 30 minutes for the node of the group to become Ready |
| `record-node` | Records the node name and its boot ID in the ConfigMap `e2e-fencing-state` of the test namespace |
| `baseline-no-fencing`, `baseline-kernel-panic` | No `spec.fencing`; `kernel.panic=10` |

For each of `normal`, `slow`, `critical` and `watchdog` (expected mode and profile: `Normal`/`normal`, `Slow`/`slow`, `Critical`/`critical`, `Slow`/`slow`):

| Step | What it checks |
| ---- | -------------- |
| `<mode>-set-mode` | Sets `spec.fencing.mode` |
| `<mode>-agent-config` | ConfigMap `fencing-agent-config` has the group with the expected mode |
| `<mode>-daemonset-armed` | The pod template of DaemonSet `fencing-agent-<group>` has `app: fencing-agent`, `node-manager.deckhouse.io/fencing-agent-version: "2"`, `hostNetwork: true` and the `watchdog` volume |
| `<mode>-daemonset-rolled-out` | The DaemonSet is rolled out: observed generation, updated and available pods |
| `normal-memberlist-key`, `normal-socket-ngc` | Only in `Normal`: Secret `fencing-memberlist-key` exists (its content is never read), NodeGroupConfiguration `fencing-agent-socket-directory.sh` exists |
| `<mode>-watchdog-ngc` | NodeGroupConfiguration `enable-watchdog-for-nodegroup-<group>.sh` selects only this group |
| `<mode>-node-armed` | The node has `Fencing=True/Armed` |
| `<mode>-node-profile` | The node annotation `fencing-profile` has the expected profile |
| `<mode>-node-cp-source` | The node annotation `fencing-cp-source` is `proxy` or `self` |
| `<mode>-kernel-panic` | `kernel.panic=0` on the node |
| `<mode>-not-rebooted` | The boot ID of the node did not change |

Maintenance:

| Step | What it does |
| ---- | ------------ |
| `maintenance-annotate` | Puts the annotation `node-manager.deckhouse.io/fencing-disable` on the node. Its cleanup removes it |
| `maintenance-node-disarmed` | `Fencing=False/Maintenance` |
| `maintenance-remove-annotation` | Removes the annotation |
| `maintenance-node-armed` | `Fencing=True/Armed` again |
| `maintenance-not-rebooted` | The boot ID did not change |

Disabled. The expected state depends on the other groups: while another group is fenced, the agents stay in the cluster.

| Step | Another group is fenced | No other group is fenced |
| ---- | ----------------------- | ------------------------ |
| `disabled-set-mode` | Sets `Disabled` | Sets `Disabled` |
| `disabled-watchdog-ngc-removed` | The watchdog NodeGroupConfiguration of the group is gone | The same |
| `disabled-agent-config` | The group is `Disabled` in ConfigMap `fencing-agent-config` | The ConfigMap is gone |
| `disabled-group-daemonset` | DaemonSet `fencing-agent-<group>` exists | It is gone |
| `disabled-agent-daemonsets` | Agent DaemonSets exist | No DaemonSet `app=fencing-agent` is left |
| `disabled-node-condition` | `Fencing=False/GroupDisabled` | No condition `Fencing` |
| `disabled-node-profile` | Annotation `fencing-profile=disabled` | No profile annotation |
| `disabled-kernel-panic` | `kernel.panic=10` | The same |
| `disabled-not-rebooted` | The boot ID did not change | The same |

Deletion:

| Steps | What they do |
| ----- | ------------ |
| `delete-group` | Deletes the test group and waits until it is gone |
| `deleted-nodes`, `-machines`, `-instances`, `-daemonset`, `-watchdog-ngc` | One step each: nothing of the test group is left (waits up to 15 minutes) |

## Files

| File | Purpose |
| ---- | ------- |
| `chainsaw-test.yaml` | Chainsaw test definition |
| `manifests/nodegroup.yaml` | The test NodeGroup, built from the instance class and zone of the source group |
| `manifests/state.yaml` | ConfigMap with the node name and the boot ID of the baseline |
| `asserts/source-group.yaml` | The source group is `CloudEphemeral` |
| `asserts/group-node-ready.yaml` | The node of the test group is Ready |
| `asserts/group-*.yaml` | Nodes, Machines, Instances, the agent DaemonSet and the watchdog NodeGroupConfiguration of the test group, for the checks that they are gone |
| `step-templates/set-mode.yaml` | Sets `spec.fencing.mode` |
| `step-templates/assert-*.yaml` | One check each, called with the expected values; `assert-kernel-panic.yaml` starts an unprivileged pod on the node that waits for the expected `kernel.panic` |

## Pass/Fail Criteria

The test passes when every step passes and the last steps find nothing of the test group. The JUnit report (`reports/chainsaw-report.xml`, format `JUNIT-OPERATION`) has one entry per operation of every step. A failed check shows the expected and the actual values; the steps that check a node or the agents also show the events of `d8-cloud-instance-manager` and the agent logs, and a failed `kernel.panic` check shows the last value read in the termination message of its pod.

## Running

```bash
export FENCING_SOURCE_NODE_GROUP=worker       # optional
export FENCING_INSTANCE_CLASS=<class-name>    # optional

# From the test directory
task run

# From the fencing-agent root
task fencing-agent-deployment:run
```

## Troubleshooting

If the test process is killed, the cleanup does not run. Delete the group by hand and wait until its node is gone:

```bash
kubectl delete nodegroup e2e-fencing
kubectl wait --for=delete nodes -l node.deckhouse.io/group=e2e-fencing --timeout=20m
```

The next run also deletes a group that a previous run left and waits until nothing of it is left.

Useful checks:

```bash
kubectl get daemonsets -n d8-cloud-instance-manager -l app=fencing-agent
kubectl get node -l node.deckhouse.io/group=e2e-fencing -o jsonpath='{.items[0].status.conditions[?(@.type=="Fencing")]}'
kubectl logs -n d8-cloud-instance-manager -l app=fencing-agent,node-group-name=e2e-fencing -c fencing-agent
```

## Safety

The test creates and deletes only its own NodeGroup and reads the source group. No step stops or restarts a node. When a step fails, the cleanup deletes the group, waits until its node is gone and removes the maintenance annotation.
