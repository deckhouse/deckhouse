# Resource measurements: methods and results

This document records how the resource consumption of `admission-policy-engine` is measured and what the
measurements produced for the release that introduced controller-level checks.
Read it before you start a new investigation: it tells you which method answers which question, which
measurements are known to be misleading, and what the current numbers are, so you do not re-derive them.

The figures are release-scoped and go stale. Treat them as a baseline to compare against, not as a
specification. When you change a ConstraintTemplate, the shared libraries under `../charts/constraint-templates/files/libs/`,
or the audit flags, re-run the affected method and update the corresponding section.

## Scope of these figures

The numbers in [Results for this release](#results-for-this-release) describe one specific state, and every
comparison depends on it.

| Property | Value |
| --- | --- |
| Change under measurement | Controller-level checks (PR #21556) |
| Measured on | 2026-09-08 and 2026-09-09 |
| Gatekeeper | 3.22.2 (`GATOR_VERSION` in the repository `Makefile` is kept in sync with it) |
| Cluster | 2 nodes, Kubernetes v1.34.11, development build of the branch |
| Module settings | `defaultPolicy: Baseline`, `enforcementAction: Deny`, `controllerValidation: true` |
| Audit flags | `--audit-interval=60`, `--audit-from-cache=false`, `--audit-chunk-size=500`, `--audit-match-kind-only=true` |
| Active constraints | 65 — 34 `deny`, 30 `warn`, 1 `scoped` |
| Violations in the cluster | 0 |
| Vertical Pod Autoscaler | disabled, so the audit deployment has requests but no limits |

Two properties of this cluster change how you read the numbers. Because there are no violations, the
per-rule figures for the compliant path apply, and those differ from the violation path by up to 3x on the
expensive rules. Because Vertical Pod Autoscaler is disabled, the audit container is not capped at the
`maxAllowed` of 500m that its VerticalPodAutoscaler resource would otherwise impose.

## Available methods

Five methods cover the module, and each answers a different question.
Pick by the question you have, because a method applied to the wrong code path produces a number that looks
authoritative and means nothing.

| Method | Answers | Cannot answer |
| --- | --- | --- |
| Per-rule benchmark | Cost of one policy evaluation on one object | Total cost of a webhook request or an audit cycle |
| One audit cycle | Exact CPU and allocations of a single audit pass | Whether that pass was typical |
| Audit duration history | Distribution of cycle durations over time | Where inside a cycle the time goes |
| Cost decomposition | Which share of a cycle the policy logic accounts for | Absolute cost, without a cycle measurement to scale it |
| A/B with a module setting | Cost of one feature, measured rather than inferred | Anything the setting does not switch off |

### Measuring the cost of one rule

Use the per-rule benchmarks to compare ConstraintTemplates against each other and against their own history.
They evaluate one policy against one object with the Open Policy Agent Go SDK, which makes them representative
of the **webhook** path, where one admission review is one object evaluation.
They are not representative of the audit path, which adds listing and bookkeeping around every evaluation.

Run the benchmark from the constraints root after the `rendered/` fixtures exist:

```bash
cd modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints
../../tools/rulebench.sh .                     # all constraints, time and memory
../../tools/rulebench.sh . allowed-users       # one constraint
python3 ../../tools/bench_rules.py . --count 500 --format table
```

Each line reports `ns/op`, `B/op`, `allocs/op` and the fixture the number came from.
Compare `allocs/op` and `B/op` only; see [Pitfalls](#pitfalls-that-invalidate-a-measurement) for why `ns/op`
is unusable here.

### Measuring one audit cycle

Use the audit cycle script to get the CPU and allocation cost of a single audit pass.
It diffs Go runtime counters across the exact cycle boundaries taken from the audit log, so it needs no
`--enable-pprof` and mutates nothing.

Run it against the target cluster:

```bash
cd modules/015-admission-policy-engine/tools
./audit_cycle_cost.sh                          # current kubectl context
./audit_cycle_cost.sh -t 300                   # raise the wait if the interval is long
```

The script waits for a new cycle to start, waits for it to finish, and prints wall time, CPU-seconds,
allocations, garbage collector cycles and the net change in live objects.
A negative net change in live objects across a cycle means nothing leaks; a value that trends up across
several runs points at an actual leak, which one cycle cannot show.

### Reading the audit duration history

Use Gatekeeper's own histogram when you need to know whether a cycle you measured was typical.
It accumulates every cycle since the audit container started, which makes it far more reliable than one or
two samples from the cycle script.

Forward the metrics port and read the histogram:

```bash
POD=$(kubectl -n d8-admission-policy-engine get pods -l control-plane=audit-controller \
  -o jsonpath='{.items[0].metadata.name}')
kubectl -n d8-admission-policy-engine port-forward "$POD" 18901:8888 &
curl -s http://127.0.0.1:18901/metrics | grep '^gatekeeper_audit_duration_seconds'
```

Divide `gatekeeper_audit_duration_seconds_sum` by `_count` for the mean cycle duration, and read the `le`
buckets for the distribution.
Compare the mean against `--audit-interval`: a mean above the interval means the audit controller is busy
most of the time and violation reporting lags.

### Decomposing the cost of a cycle

Use the decomposition when you need to know whether the policy logic or the surrounding framework dominates a
cycle, because that determines whether optimising Rego can help at all.
The audit loop evaluates one (object, constraint) pair at a time, so divide the measured allocations of a
cycle by the number of pairs it actually evaluated.

Count the pairs rather than multiplying objects by constraints.
Namespace and label selectors reduce the product by an order of magnitude, so the naive product overstates
the work by that much.
For each constraint, count the objects that match all three of its `match.kinds`, `match.namespaceSelector`
and `match.labelSelector`, then sum over constraints.

Compare the resulting allocations per pair against the per-rule benchmark for the same constraint kinds,
weighted by how many pairs each kind gets.
The difference is Gatekeeper's own per-evaluation overhead: listing, deep copies, review construction, and
status and violation bookkeeping.

### Measuring one feature with an A/B run

Use an A/B run when you need the cost of a specific feature instead of an estimate derived from its share of
the work.
For controller-level checks, `controllerValidation: false` removes the controller kinds from every constraint,
so the audit loop stops walking them, which isolates the audit-side cost of the feature.

Create the ModuleConfig, wait for the constraints to re-render, measure, then delete it:

```bash
kubectl apply -f - <<'EOF'
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: admission-policy-engine
spec:
  version: 1
  enabled: true
  settings:
    podSecurityStandards:
      controllerValidation: false
EOF
# Wait until match.kinds contains Pod only, then run audit_cycle_cost.sh.
kubectl get d8hostnetwork d8-pod-security-baseline-deny-default -o jsonpath='{.spec.match.kinds}'
kubectl delete moduleconfig admission-policy-engine
```

The constraints re-render within about 15 seconds, and deleting the ModuleConfig restores the previous state.
Leave `defaultPolicy` out of the ModuleConfig on purpose: the `detect_pss_default_policy` hook derives it
whenever configuration does not set it, so omitting it keeps the cluster on the policy it already had.

Always confirm the restore. Check that the ModuleConfig is gone, that every constraint matches Deployment
again, and that no constraint reports an error.

## Pitfalls that invalidate a measurement

Every pitfall below produced a wrong number during this investigation before it was caught.
Check each one before you trust a comparison.

- **The benchmark picks its own fixture.** Both benchmark tools select the "allowed" and "disallowed" samples
  by filename from `rendered/test_samples/**` in sorted order. A file named `001-controller-allowed-*.yaml`
  sorts first and contains "allowed", so a constraint with controller test cases is benchmarked against a
  Deployment review by default. Comparing that against a historical Pod number compares two different inputs.
  Move the controller samples aside first; `../AGENTS.md` carries the exact commands.
- **Timings are not comparable between runs.** On a workstation the same configuration varied by up to 7x
  between consecutive runs: `deny-exec-heritage` on the violation path measured 356k, 522k and 72k `ns/op`
  with an identical 783 `allocs/op`. Use `allocs/op` and `B/op`, which reproduce to under 1%, and treat
  `ns/op` as an order-of-magnitude signal only.
- **Cost per pair is not uniform, so shares cannot be inferred from counts.** Controller kinds account for
  30% of the evaluated pairs, and projecting from that ratio predicted a 30% saving. The measured saving was
  15% of CPU. Pods dominate a cycle out of proportion to their share of pairs, and part of every cycle is
  fixed work that does not scale with objects at all.
- **A SecurityPolicy renders more constraints than it names.** `allow_privileged` and
  `allow_privilege_escalation` are rendered for every SecurityPolicy that does not set the corresponding
  policy to `true`. A test policy that names only the field under test will therefore deny fixtures for an
  unrelated reason. Set both to `true` in test policies.
- **Pod Security Standards constraints apply to the test namespace too.** With `defaultPolicy: Baseline`, the
  generated constraints match every namespace that is not labelled
  `security.deckhouse.io/pod-policy: privileged`, so a denial may come from them rather than from the policy
  under test. Label the test namespace `privileged`, and attribute every denial by the constraint name that
  the message is prefixed with.
- **The duration histogram resets.** It lives in the audit container's memory, so a container restart zeroes
  it. Read `_count` before and after any long observation and discard the interval if the count decreased.

## Results for this release

The subsections below record what the methods produced on the cluster described in
[Scope of these figures](#scope-of-these-figures).

### Cost of one rule on the Pod path

Controller-level checks left the Pod path almost unchanged.
Across 78 comparisons on identical Pod fixtures, 53 are unchanged within 1% and 23 regressed by 5% to 70%.

| Constraint and path | Before the change | After |
| --- | --- | --- |
| `allowed-users`, violation | 250 allocs/op | 424 (+70%) |
| `vulnerable-images`, compliant | 407 allocs/op | 587 (+44%) |
| `allowed-host-paths`, violation | 925 allocs/op | 1 252 (+35%) |
| `verify-image-signature` | 523 allocs/op | 630 (+20%) |
| `allow-privileged`, compliant | 504 allocs/op | 599 (+19%) |
| `allowed-proc-mount`, compliant | 655 allocs/op | 754 (+15%) |
| `allowed-proc-mount`, violation | 1 933 allocs/op | 1 941 (+0.4%) |
| `allow-privileged`, violation | 1 432 allocs/op | 1 439 (+0.5%) |
| `allowed-repos`, compliant | 231 allocs/op | 231 (unchanged) |

The most expensive rules barely moved, so the ranking of rules by cost did not change.
The allocation floor of the Pod path did not move either: the cheap rules stay at about 230 `allocs/op`.
The floor of about 270 `allocs/op` belongs to a controller review, which is a new kind of review rather than
a more expensive existing one.

### Cost of a controller review

A controller review costs a median of 12% more allocations than the same rule applied to a Pod, measured
across 40 rule and path combinations.
The extra cost is the pod-template resolution in `lib.common`.

### Cost of one audit cycle

Three cycles were measured at two cluster sizes, with controllers audited.

| Cycle | Objects | Wall time | CPU-seconds | Allocations | Garbage collector cycles |
| --- | --- | --- | --- | --- | --- |
| 1 | 294 | 27.4 s | 26.6 | 38.76 M | 26 |
| 2 | 294 | 44.0 s | 29.0 | 38.81 M | 27 |
| 3 | 411 | 80.0 s | 36.9 | 48.16 M | 31 |

Allocations are reproducible to 0.1% at a fixed object count and scale close to linearly with the number of
objects.
Wall time is not reproducible, which is why the history below is the figure to quote.
Nothing leaks: the net change in live objects came out negative on two of the three cycles.

### Audit duration over 60 cycles

The histogram is the reliable statement about cycle duration, and it shows the audit loop exceeding its own
interval most of the time.
Over 60 cycles the sum was 5 562.9 seconds, so the mean cycle took 92.7 seconds against an
`--audit-interval` of 60.

Only 18 of the 60 cycles finished within the interval, 39 took between 60 and 180 seconds, and 3 took up to
300 seconds.
Cycles do not pile up, because Gatekeeper sleeps for the interval after a cycle finishes, but the audit
controller is busy roughly 60% of wall time and violation reporting for existing workloads lags by up to
about 2.5 minutes.

### Where the cost of a cycle goes

The policy logic is a rounding error in the audit cost, and the framework around it dominates.
At 411 objects and 65 constraints, selectors reduce the product of 411 by 65 to 3 515 evaluated pairs, of
which 2 470 are Pod pairs and 1 045 are controller pairs.

A cycle of 48.16 M allocations over 3 515 pairs is about 13 700 allocations per pair.
Weighting the per-rule benchmarks by the pairs each constraint kind actually receives accounts for about
1.25 M allocations, which is under 3% of the cycle.
The remaining 97% is Gatekeeper's own per-evaluation overhead, which no change to the policies can move.

### Cost of controller-level auditing

The A/B run measured the feature directly instead of inferring it from the 30% pair share.

| One audit cycle | Controllers audited | Pod only |
| --- | --- | --- |
| Allocations | 48.16 M | 43.92 M (−8.8%) |
| CPU-seconds | 36.9 | 31.3 (−15%) |
| Wall time | 80.0 s | 59.1 s |

Controller-level auditing costs about 9% of the allocations and 15% of the audit CPU, which is roughly
5.6 CPU-seconds per cycle on this cluster.

The Pod-only cycle still took 59 seconds of wall time and 31 CPU-seconds, so the audit loop exceeded its
interval before controller-level checks existed.
The overrun is a property of this constraint set, not of the feature.

## Conclusions and rejected optimisations

The measurements settle three questions about this release.

The policy logic is not the bottleneck. Rego accounts for under 3% of an audit cycle, and the branch's own
per-rule numbers moved by well under 1% on the most expensive templates. Optimising Rego further cannot
change the resource picture.

Controller-level checks are affordable. They add 15% to audit CPU and a median of 12% to the cost of a single
review, and the reviews they add are new rather than more expensive versions of existing ones.

The audit overrun is inherited, not introduced. A Pod-only cycle already exceeds `--audit-interval` on a
411-object cluster, so any work on the overrun belongs outside this feature. The levers are the audit flags
and the number of active constraints, not the policies.

One optimisation was evaluated and rejected.
Gatekeeper supports restricting a constraint to a single enforcement point, and the platform already uses it:
`operator-trivy` renders its `vulnerable-image` constraint with `enforcementAction: scoped` and
`enforcementPoints: [validation.gatekeeper.sh]`, which keeps the webhook check and excludes the constraint
from audit.
Auditing controllers is redundant, because a controller that violates a policy produces Pods that violate it
too and audit already reports those Pods.

The obstacle is not support but value.
Because `scopedEnforcementActions` applies to a whole constraint rather than to individual kinds, every
pod-spec constraint would have to be rendered twice, raising the constraint count from 65 to about 99 along
with their status resources, and controller entries would disappear from audit reports.
The measured return for that is the 15% of audit CPU above.
The trade was judged not worth making; revisit it only if the audit flags are retuned first and 15% still
matters afterwards.

## Re-running these measurements

Update this document when a change can move the numbers it records.
The trigger is a change to a ConstraintTemplate, to a shared library under `../charts/constraint-templates/files/libs/`,
to the set of constraints the module renders, or to the audit flags in `../templates/audit-deployment.yaml`.

Re-run the methods that the change can affect:

1. For a rule or library change, re-run the per-rule benchmark on Pod fixtures and update
   [Cost of one rule on the Pod path](#cost-of-one-rule-on-the-pod-path).
1. For a change to the rendered constraint set, re-count the evaluated pairs and update
   [Where the cost of a cycle goes](#where-the-cost-of-a-cycle-goes).
1. For an audit flag change, re-read the duration history and update
   [Audit duration over 60 cycles](#audit-duration-over-60-cycles).

State the cluster the new figures came from in [Scope of these figures](#scope-of-these-figures), because a
number without its cluster is not comparable to anything.
