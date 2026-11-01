# Webhook on a master under a noisy neighbour

The scenario checks that the cert-manager webhook keeps admitting requests when another pod takes the CPU of its master.

## Summary

A stress pod on the master that runs the webhook requests all free allocatable CPU of the master minus 100m
and runs one busy loop per CPU (`threadsFactor: 1`).
For the whole test, a loop creates a Certificate and a CertificateRequest with `--dry-run=server` about twice a second.
Both requests pass the webhooks of cert-manager, which use `failurePolicy: Fail`:
the CertificateRequest passes the mutating and the validating webhook, the Certificate passes the validating webhook.
A burst of `certificateCount` Certificates issued under the pressure also goes through the webhook,
because every write of the controller to a cert-manager resource is admitted by it.

With several masters, the webhook runs one replica per master and the Service spreads the requests among them,
so only the share of requests that reaches the stressed replica is slowed down.

## Test steps

Most steps come from the step templates in `../common/step-templates/`:

| Step | Description |
| --- | --- |
| `preflight` | Labels the test namespace, checks the module is Ready, creates the ClusterIssuer, selects a webhook pod, records restarts and leases |
| `start-admission-loop` | Starts the dry-run admission loop and waits for its first successful request |
| `baseline` | Starts the watchers, issues the baseline burst, waits until `holdSeconds` have passed, marks the end of the baseline |
| `pressure` | Marks the start of the pressure, starts the stress pod on the master, issues the burst, holds the pressure for `holdSeconds` and until the burst is Ready |
| `release` | Deletes the stress Deployment |
| `admission-recovers` | Waits until a dry-run Certificate passes admission, at most `recoverySeconds` after the release |
| `recovery` | Issues three Certificates and waits until they and all module Deployments are Ready, at most `recoverySeconds` after the release |
| `after-window` | Waits `afterSeconds` and compares the write rates with the baseline |
| `admission-check` | Stops the loop and checks the share of failed admission requests in each window |
| `final-checks` | Compares restart counts, pods and leases with the preflight snapshot |

The cleanup stops the admission loop and the watchers and deletes the stress Deployment,
the test namespace and the ClusterIssuer.

## Pass criteria

The scenario passes when all of the following holds:

- in the baseline, under pressure and after it, at most `maxAdmissionFailurePercent` (default 1) of the
  dry-run admission requests failed, and each window has requests;
- every admission request reached the API server; a request that did not (no kubeconfig, no connection,
  failed authentication) fails the test as an infrastructure failure and is not counted as an admission failure;
- a dry-run Certificate passes admission within `recoverySeconds` (default 120) after the release;
- all Certificates of the burst under pressure are Ready within `burstBudgetSeconds` (default 900);
- the three recovery Certificates and all Deployments of the module are Ready within `recoverySeconds` after the release;
- the write rates under pressure and after it do not exceed the baseline as described in `../../README.md`;
- no container of `d8-cert-manager` restarted, no pod was replaced, no lease changed its holder or `leaseTransitions`.

On a stand with an 8-CPU master, 4 CPU of which were requested, the webhook served the requests
under this pressure without failures and without restarts.
The admission summary of the run reports the longest request per window.

## Running

Run the scenario from its directory:

```bash
task run
task run -- --set holdSeconds=900 --set maxAdmissionFailurePercent=0
```

The scenario starves a master of CPU, which slows down the control plane components on it as well.
