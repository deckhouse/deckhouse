# Controller under a noisy neighbour

The scenario checks that the cert-manager controller keeps working when another pod takes the CPU of its node.

## Summary

A stress pod on the node of the controller leader requests all free allocatable CPU of the node minus 100m
and runs one busy loop per node CPU (`threadsFactor: 1`).
The controller keeps its own small CPU request and gets only a tiny share of the CPU by weight.
Under this pressure, a burst of `certificateCount` Certificates must become Ready within 15 minutes,
the controller must not restart or lose its lease, and its write rate must not grow.

## Test steps

The steps come from the step templates in `../common/step-templates/`:

| Step | Description |
| --- | --- |
| `preflight` | Labels the test namespace, checks the module is Ready, creates the ClusterIssuer, selects the leader pod, records restarts and leases |
| `baseline` | Starts the watchers, issues the baseline burst, waits until `holdSeconds` have passed, marks the end of the baseline |
| `pressure` | Marks the start of the pressure, starts the stress pod on the leader node, issues the burst, holds the pressure for `holdSeconds` and until the burst is Ready |
| `release` | Deletes the stress Deployment |
| `recovery` | Issues three Certificates and waits until they and all module Deployments are Ready, at most `recoverySeconds` after the release |
| `after-window` | Waits `afterSeconds` and compares the write rates with the baseline |
| `final-checks` | Compares restart counts, pods and leases with the preflight snapshot |

The cleanup stops the watchers and deletes the stress Deployment, the test namespace and the ClusterIssuer.

## Pass criteria

The scenario passes when all of the following holds:

- all Certificates of the burst under pressure are Ready within `burstBudgetSeconds` (default 900);
- the three recovery Certificates and all Deployments of the module are Ready within `recoverySeconds` (default 120)
  after the release;
- the baseline has `MODIFIED` events of the leases and at least `certificateCount` `MODIFIED` events of the Certificates;
- the rate of `MODIFIED` events of the leases and of the Certificates under pressure and after it
  is at most twice the baseline rate or at most the baseline rate plus 10 per minute;
- no container of `d8-cert-manager` restarted, no pod was replaced, no lease changed its holder or `leaseTransitions`.

On a stand with 6-CPU workers and `certificateCount=50`, 50 Certificates became Ready in 520 s under this pressure against 68 s without it.

## Running

Run the scenario from its directory:

```bash
task run
task run -- --set holdSeconds=900
```

The node of the leader must have at least 200m of unrequested CPU, otherwise the `pressure` step fails.
