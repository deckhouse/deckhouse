# Webhook under a high load average with a CPU limit

This scenario is the `webhook-high-load-average` scenario with a CPU limit equal to the raised request,
which adds CFS quota throttling to the run queue shared with many stress pods.

## Summary

The target is the `manager` container of `gatekeeper-controller-manager`.
The `resize-target` step raises the CPU request to the current usage and sets the CPU limit to the same value in place.
Within every 100 ms CFS period the container can then use no more CPU time than its request.
Unlike the run queue contention, this throttling applies to the container whatever runs next to it.

Gatekeeper sizes its serving semaphore from `GOMAXPROCS` once at start,
so a limit set in place does not reduce the number of concurrent evaluations.

A CPU limit cannot be removed in place.
The cleanup therefore restarts `deployment/gatekeeper-controller-manager` with `kubectl rollout restart`
and waits for the rollout: the new pod gets the resources of the Deployment template.
With one replica, the Deployment uses the default rolling update and starts the new pod before it stops the old one.
In the high availability mode, the Deployment has two replicas with `maxSurge: 0` and `maxUnavailable: 1`:
it stops one old pod before it starts its replacement, and the second replica serves the webhook meanwhile.

## Test steps

The steps are the same as in `webhook-high-load-average`.
The only difference is the `CPU_LIMIT=true` setting of the `preflight` step.

## Pass and fail criteria

The criteria are the same as in `webhook-high-load-average`.
The restart and replacement check runs before the cleanup, so the restart made by the cleanup does not count.

## Running

Run the scenario from this directory:

```bash
task run
```

After the run, check that the webhook pod has no CPU limit again:

```bash
kubectl -n d8-admission-policy-engine get pod -l control-plane=controller-manager \
  -o jsonpath='{.items[*].spec.containers[?(@.name=="manager")].resources}'
```

The expected output has no `limits.cpu` unless the Deployment template sets one.
After an interrupted run, `task cleanup` restarts the Deployment to drop the limit.
