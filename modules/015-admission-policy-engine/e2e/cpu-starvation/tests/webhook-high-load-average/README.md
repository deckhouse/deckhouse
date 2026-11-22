# Webhook under a high load average

This scenario checks that the Gatekeeper webhook keeps working when it keeps the CPU share of its request,
but competes for the run queue with many stress pods.

## Summary

The target is the `manager` container of `gatekeeper-controller-manager`.

The scenario first raises the CPU request of the container in place to its current usage
(at least `100m`, measured with `kubectl top` while the probes run):

```bash
kubectl patch pod <POD> --subresource resize \
  -p '{"spec":{"containers":[{"name":"manager","resources":{"requests":{"cpu":"<CPU>"}}}]}}'
```

Here `<POD>` is the webhook pod and `<CPU>` is the new request.
The resize needs Kubernetes 1.33 or newer and does not restart the container.
The `freeze-vpa` step has already switched the VerticalPodAutoscaler of the Deployment to `updateMode: Off`,
so it does not undo the change.

The CPU of the node is shared between cgroups by weight, and every pod is a cgroup of its own.
Burner threads inside one pod would not take a larger share from the webhook and would barely delay it,
so the scenario spreads the load over many pods.
The stress Deployment of the node runs up to `maxStressPods` (100) small Burstable pods
whose requests add up to the free CPU of the node, each with 2 to 4 burner threads,
up to `threadsFactor` (60) threads per node CPU in total.
The number of pods is also limited by the free pod slots of the node minus a reserve of 10.
The suite README describes the choice in [Stress pods](../../README.md#stress-pods).

The webhook keeps the CPU share of its raised request, but every stress pod is another runnable cgroup
in the run queue of the node, so the webhook waits longer for every wake-up.
The load average of the node grows to the number of burner threads, several hundred on a node with 4 or more CPU.

## Test steps

The scenario runs the following steps:

| Step | Description |
| ---- | ----------- |
| `preflight` | Labels the test namespace; checks the module, Kubernetes 1.33+ and `kubectl patch --subresource` |
| `freeze-vpa` | Sets `updateMode: Off` on the VPA of `gatekeeper-controller-manager`; the cleanup restores it |
| `start-observers` | Takes snapshots and starts the probe loop, the watches and the webhook configuration poll |
| `baseline` | Collects the baseline for `baselineWindow` (3m) |
| `resize-target` | Raises the CPU request of `manager` to its usage in place; the cleanup restores the original request in place |
| `start-pressure` | Starts the stress Deployment: up to 100 pods with the rest of the node, 2 to 4 threads each |
| `hold-pressure` | Keeps the pressure for `hold` (5m) |
| `release-and-recover` | Deletes the stress Deployment and waits up to `recovery` (120s) |
| `observe-after-release` | Observes for `afterWindow` (5m) |
| `analyse` | Asserts the results and prints the summary |

## Pass and fail criteria

The criteria are the same as in `webhook-noisy-neighbour`: at most 1% of failed probes, no admitted privileged Pod,
recovery within `recovery`, no write storm, no restarts, and the webhook configuration never disappears.
A restart of the container during the resize itself is reported as a warning and does not count:
the restart snapshot is taken again after the resize.

## Running

Run the scenario from this directory:

```bash
task run
```

Run it with fewer stress pods for a small node:

```bash
task run -- --set maxStressPods=30
```

After an interrupted run, remove the stress pods and restore the webhook pod with `task cleanup`.
