# Diagnosing resource usage in a live cluster

> If you're an AI agent picking up a resource-consumption investigation on
> this module, read [`AGENTS.md`](AGENTS.md) first - it has the established
> findings (what's already known, what was tried and rejected) so you don't
> re-derive them from scratch.

Two read-only scripts for a **running** admission-policy-engine (Gatekeeper)
deployment:

- [`live_resource_check.sh`](live_resource_check.sh) — broad sweep: pods,
  restarts, resource usage over time, Gatekeeper's own metrics, cluster-wide
  scale and events. Start here.
- [`audit_cycle_cost.sh`](audit_cycle_cost.sh) — narrow and precise: measures
  the CPU-seconds and memory allocations actually consumed by **one specific
  audit cycle**, by diffing Go-runtime counters across that cycle's exact
  start/end (known from the audit log). No `--enable-pprof`, no Deployment
  patch, no restart — see "Attributing cost without pprof" below for why this
  works and what it can/can't tell you compared to a real profiler.

Together they answer "why is this module using this much CPU/memory *right
now, in this cluster*?" and, just as importantly, help you tell apart three
different kinds of cause before you touch anything:

- **fixable** — controlled by a flag or CR this module owns, safe to change;
- **architectural** — inherent to how Gatekeeper/OPA works (e.g. full-cluster
  Pod sync for a built-in constraint, full API discovery on every audit
  cycle) — not something a config change in this module can remove;
- **environmental** — the cluster/node is under pressure (disk, CPU, network)
  and this module is just one of many casualties, not the cause.

This is the live-cluster counterpart to the static per-rule benchmarks in
[`../charts/constraint-templates/tests/tools/`](../charts/constraint-templates/tests/tools)
(`bench_rules.py`, `rulebench`) — those measure a single Rego rule in
isolation against a synthetic test object; this script measures the actual
running Deployment against real cluster state. Use the live check first to
see *whether* there's a real problem and roughly *where* it comes from (an
expensive rule vs. cluster size vs. node pressure); if it points at specific
ConstraintTemplates, use `rulebench`/`bench_rules.py` to quantify which one
and by how much.

## Usage

```bash
cd modules/015-admission-policy-engine/tools
./live_resource_check.sh                                  # defaults, current kube-context
./live_resource_check.sh -n d8-admission-policy-engine -s 5 -i 20 -o ./diag
```

| Flag | Meaning | Default |
| ---- | ------- | ------- |
| `-n` | namespace to inspect | `d8-admission-policy-engine` |
| `-s` | number of `kubectl top` samples | `3` |
| `-i` | seconds between samples | `15` |
| `-o` | output directory (report + raw data) | `./ape-diag-<timestamp>` |

Requires `kubectl` (against an already-selected context — the script never
switches context or namespace scope beyond what you pass in), `awk`, `curl`.
`python3` is used opportunistically for one JSON breakdown and skipped
gracefully if absent. No cluster-admin needed beyond what you already use to
read the namespace and port-forward to a pod you can already `get`/`logs`.

It writes a full text report plus the raw data it collected (pod specs,
`kubectl top` samples, one Prometheus scrape per pod, the audit log tail, and
recent cluster-wide Warning events) to the output directory, so you can
re-inspect anything after the fact without re-running against the cluster.

## What each section checks, and how to read it

**1. Pods, restarts, last-restart reason.** `exitCode=0` + `reason=Completed`
on a restart means the container was killed by a failed liveness/readiness
probe — it did not crash on its own. That's a symptom to explain via the
other sections (was it CPU/memory-starved at that moment? was the whole node
in trouble?), not a bug in the container itself.

**2. Requests vs. limits.** A blank limits column means the container can
grow unbounded relative to its request whenever the `vertical-pod-autoscaler`
module is disabled — expected in some module versions, but worth confirming
before treating usage above the request as a leak.

**3. Resource usage over N samples.** A single `kubectl top` snapshot can't
distinguish a periodic spike from a sustained baseline. A wide CPU range with
a flat memory range points at something periodic (an audit cycle — check
section 5); memory sitting well above the request even at the *low* end of
the range is a steady-state base cost, not a spike (see sections 4/6 for what
makes up that base).

**4. Gatekeeper's own Prometheus metrics**, fetched by port-forwarding
straight to each pod's `:8888` (bypassing `kube-rbac-proxy`'s auth — fine for
read-only diagnosis). Key numbers:
  - `gatekeeper_sync{kind=...}` — how many objects of each kind are cached in
    memory. A large count for a kind you didn't expect (typically `Pod`)
    scales with **total cluster size**, not with how many
    SecurityPolicy/OperationPolicy CRs exist — it usually means a built-in
    constraint needs that kind unconditionally, not a misconfiguration.
  - `gatekeeper_audit_duration_seconds_{sum,count}` — average audit-cycle
    duration (`sum/count`); compare against the audit interval (section 5).
  - `gatekeeper_constraints{enforcement_action=...}` — how many Constraint
    objects are actually active, broken down by enforcement action.
  - `process_resident_memory_bytes`, `go_memstats_heap_alloc_bytes`,
    `go_goroutines` — baseline process health.

  Note what's *not* here: no per-rule cost. Gatekeeper's metrics are
  aggregates across every active constraint in one audit run / webhook call
  — if a specific rule looks implicated, quantify it with `rulebench`/
  `bench_rules.py` instead of trying to read it off these numbers.

**5. Audit-cycle timing from logs.** Parses `audit_started`/`audit_finished`
log lines for cycle count and duration, and flags any cycle that ran 3×+
longer than the average — a genuine outlier worth correlating with section 1
restarts and section 7 cluster-wide events, not just averaged away. Compare
the average cycle time with `--audit-interval` in the audit Deployment's
args: if the average is a large fraction of the interval, the audit loop is
running almost back-to-back.

**6. Cluster-wide scale.** Total pods/namespaces/CRDs and node pressure
conditions — the things this module's cost structurally scales with
(discovery cost scales with CRD count; sync cost scales with cluster size),
independent of any misconfiguration.

**7. Cluster-wide Warning events.** If Warnings cluster around the same
timestamps across many unrelated namespaces, that's a node/API-server-wide
problem (disk pressure, CPU steal, network) — not a defect in
admission-policy-engine specifically, even if this module's pods are among
the ones that got restarted. If Warnings are concentrated in this module's
namespace alone, it's more likely actually about this module.

## Worked example: adyakonov-cloud

Run against a real (small, 2-node, 143-pod) cluster:

```
## 1. Pods, restarts, last-restart reason
gatekeeper-audit-...     3/3   Running   14 (30m ago)   18h
gatekeeper-controller-manager-...   2/2   Running   1    18h
  gatekeeper-audit-.../manager: restartCount=8 lastReason=Completed exitCode=0

## 3. Resource usage over time
  gatekeeper-audit-.../manager   cpu 249-249m   mem 210-225Mi   (request: 100m/128Mi)
  gatekeeper-controller-manager-.../manager   cpu 47-47m   mem 219-228Mi

## 4. Gatekeeper's own metrics
  gatekeeper-audit: watched GVKs 41, audit runs 30, avg dur 16.35s
    gatekeeper_sync: Pod=143, SecurityPolicyException=24
    gatekeeper_constraints: deny=34 active, warn=30 active

## 5. Audit-cycle timing
  cycles=19  avg=16.43s  max=46.60s  max/avg=2.8x

## 6. Cluster-wide scale
  pods=143  namespaces=38  nodes=2  CRDs=299
  SecurityPolicy CRs=2  OperationPolicy CRs=0
  node pressure: all False

## 7. Cluster-wide warning events
  Distinct namespaces with recent Warning events: 12
  -> node-wide FreeDiskSpaceFailed/ImageGCFailed events present at the
     same time as this module's restarts
```

Reading this the way the sections above intend:

- Memory (210–228Mi) sits well above the 128Mi request on **both** pods, in a
  cluster with only 143 pods and 2 SecurityPolicy CRs — that's steady-state
  base cost (compiled ConstraintTemplates + the full-cluster Pod sync
  required by the built-in exec-heritage constraint + the 299-CRD discovery
  tree), not a leak specific to this deployment. Section 2 also shows no
  memory limit is set, so nothing bounds it.
- CPU on the audit pod is a flat 249m across both samples here (not spiky in
  this particular window), but section 5's log-based trend shows the real
  picture: cycles normally take ~16s (already a third of the *pre-fix*
  60-second `--audit-interval`), with one outlier at 46.6s (2.8× average).
- Section 1's restarts are `exitCode=0`/`Completed` — probe kills, not
  crashes. Section 7 shows Warning events at the same time spread across 12
  unrelated namespaces, correlated with node-level `FreeDiskSpaceFailed` —
  i.e. the restarts are largely **environmental** (this dev node was low on
  disk and image GC was thrashing), not a defect in admission-policy-engine.
  The elevated *steady-state* memory and the audit-cycle length, on the other
  hand, are real findings independent of that — see the module's
  resource-audit report for the fixes (`--audit-interval` reduced to 300s,
  explicit `limits` added matching the existing VPA `maxAllowed`).

This is exactly the split this script is for: one run separated "real,
fixable-or-architectural cost" from "noisy dev-cluster environment" without
having to eyeball raw `kubectl top`/`describe`/`events` output by hand.

## Attributing cost without pprof: `audit_cycle_cost.sh`

Gatekeeper's binary supports `-enable-pprof`/`-pprof-port` (real CPU/heap
profiles, function-level), but that flag is off by default in this module's
chart, and turning it on means patching the live Deployment and restarting a
pod — a real, if low-risk and reversible, cluster change that not everyone
running this against a shared/production cluster wants to make just to look.

There's a way to get a genuine, *measured* (not estimated) CPU and memory
number for one specific audit cycle using only what's already exposed, with
zero mutation: `process_cpu_seconds_total` and `go_memstats_mallocs_total`
are monotonic counters, and the audit log already marks each cycle's exact
start and end (`audit_started`/`audit_finished`, sharing an `audit_id`).
Snapshot `/metrics` right at both boundaries and diff the counters, and the
delta is an exact accounting of what that one cycle cost the process — no
sampling error, no averaging across idle time.

```bash
cd modules/015-admission-policy-engine/tools
./audit_cycle_cost.sh                      # waits for the next cycle, measures it
./audit_cycle_cost.sh -n <namespace> -t 90 # -t: seconds to wait for a cycle to start
```

This is necessarily coarser than a real profiler: it attributes cost to "one
audit cycle as a whole," not to a specific function or ConstraintTemplate.
For that finer breakdown without pprof either, cross-reference the numbers
it produces with the already-active `gatekeeper_sync`/`gatekeeper_constraints`
counts (from `live_resource_check.sh` section 4) and the per-rule `ns/op`/
`B/op` numbers from `rulebench` for the same active kinds — see the worked
example below for how well that cross-check actually lines up in practice.

### Worked example: one real audit cycle on adyakonov-cloud

```
cycle started ("audit_id":"2026-09-01T10:42:16Z") @ 10:42:16
cycle finished @ 10:42:23 (logged duration "7.548640275s")

wall time                 6.99s
CPU-seconds consumed      9.310s  (avg 1.33 cores during the cycle)
heap alloc, net change    -12.1 MiB
RSS, net change           +24.1 MiB
allocations (mallocs)     18 764 834 objects
frees                     18 664 999 objects
net live objects          +99 835
GC cycles triggered       15
goroutines, net change    +1 (894 -> 895)
```

Nearly **19 million allocations and 15 full GC cycles in under 7 seconds**,
using on average 1.33 CPU cores — for one audit pass over a cluster with only
143 pods. That's the concrete, measured answer to "what is the audit loop
actually doing to CPU and memory" for this specific run, without touching a
single flag.

It also cross-validates the static `rulebench` numbers from a completely
independent measurement path: at the time of this run, `gatekeeper_sync`
reported 145 synced Pods and `gatekeeper_constraints` reported 64 active
constraints (34 `deny` + 30 `warn`). 145 × 64 = 9 280 is a rough upper bound
on (pod, constraint) evaluation pairs for this cycle (not every constraint
matches every pod, so the real count of *actual* evaluations is somewhat
lower) — dividing the observed allocation count by that upper bound gives
**~2 020 allocations per (pod, constraint) pair**, which lands almost exactly
in the range `rulebench` measured in isolation for the most expensive
templates (`allowed-proc-mount` disallowed: 1 934 allocs/op; `allow-privileged`
disallowed: 1 432 allocs/op — see the main resource-audit report). Two
unrelated methods — a synthetic per-rule Go benchmark, and a live-cluster
counter diff — converge on the same order of magnitude, which is a much
stronger signal than either one alone that the SPE-heavy rules really are
where this module's CPU/memory goes.

A caveat worth stating plainly: this run happened to land on a normal-length
cycle (avg logged duration on this cluster was ~8-16s; see
`live_resource_check.sh` section 5). Re-run this a few times across different
cycles before treating one sample as "the" cost — the earlier resource-audit
report also caught an 8x outlier cycle (58s vs. ~8s normal) correlated with
node disk pressure, which a single audit_cycle_cost.sh run right before or
after that spike would have measured very differently.

## Estimating average CPU for a different cluster size

`audit_cycle_cost.sh` gives an exact number for the cluster you run it
against. To reason about a cluster you *don't* have shell access to (or to
sanity-check a resource request/limit before deploying), it's useful to
scale that one measurement into a rough model. Two different models, because
`gatekeeper-audit` and `gatekeeper-controller-manager` scale with different
things.

### `gatekeeper-audit`: scales with pod count, not with request rate

Audit is periodic and full-cluster, so its cost scales with **how much
there is to sync and re-check** (dominated by pod count, since the built-in
`D8DenyExecHeritage` constraint forces a full-cluster Pod sync regardless of
configuration - see the main resource-audit report). It does *not* scale
with admission traffic at all.

```
k = (measured CPU-seconds per cycle) / (pod count at measurement time)
avg_millicores(pods, audit_interval_s) = 1000 * k * pods / audit_interval_s + idle_floor_millicores
```

Calibration point from adyakonov-cloud (145 pods, 64 active constraints,
measured via `audit_cycle_cost.sh`): 9.31 CPU-s/cycle → `k ≈ 0.0642`
CPU-s per pod per cycle. Idle floor (CPU between cycles, from
`live_resource_check.sh` section 3) ≈ 15m. Projected:

| pods | CPU-s/cycle | avg core @ 60s interval | avg core @ 300s interval |
| ---: | ---: | ---: | ---: |
| 150 | 9.6s | 176m | 47m |
| 500 | 32.1s | 550m | 122m |
| 1000 | 64.2s | 1085m | 229m |
| 2500 | 160.5s | 2690m | 550m |
| 5000 | 321.0s | 5.4 cores | 1085m |
| 10000 | 642.1s | 10.7 cores | 2.2 cores |

Two things worth reading off this table directly:

- The `--audit-interval` 60s→300s change (see the main resource-audit
  report) is worth almost exactly the same **~4.3x** reduction in average
  audit CPU at *every* cluster size - it's a multiplier on the whole model,
  not a fixed offset.
- **On this module's own earlier patch**: the static CPU `limit` set for
  `gatekeeper-audit` (`1000m`, mirroring the webhook's own VPA `maxAllowed`
  - see `../charts/constraint-templates/templates/audit-deployment.yaml`)
  is already at or below the projected *average* (not peak) usage for
  clusters above ~2500-5000 pods, even after the interval fix. On a cluster
  that size, either enable the `vertical-pod-autoscaler` module for
  admission-policy-engine, or raise that static limit and re-measure with
  `audit_cycle_cost.sh` rather than trusting the extrapolation blindly.

### `gatekeeper-controller-manager` (webhook): scales with admission-request rate, not cluster size

The webhook evaluates one object per admission review, with no discovery or
LIST call - so `rulebench`/`bench_rules.py` numbers directly apply here
(unlike for audit, see the "Measuring per-rule cost" section of
`../charts/constraint-templates/tests/README.md`).

```
per_request_ms ≈ (distinct matching ConstraintTemplate kinds) × (typical rulebench ns/op) / 1e6
avg_millicores(creates_per_sec) = creates_per_sec * per_request_ms + idle_floor_millicores
```

With ~14 distinct `D8*` kinds actually instantiated on adyakonov-cloud and a
typical `rulebench` "allowed" cost of ~15-25µs (most templates; a handful of
SPE-heavy ones run 40-150µs), per-request cost is roughly 0.25-0.85ms.
Observed idle floor (watch/reconcile/leader-election overhead across 41
watched GVKs, independent of admission traffic) ≈ 25m:

| pod creates/sec | webhook-driven avg core | + idle floor | total avg |
| ---: | ---: | ---: | ---: |
| 0.02 (very quiet) | ~0m | 25m | ~25m |
| 1 | 0.3m | 25m | ~25m |
| 5 | 1.3m | 25m | ~26m |
| 20 (very busy) | 5m | 25m | ~30m |

The takeaway: **`gatekeeper-controller-manager`'s average CPU is dominated
by its fixed ~25m watch/reconcile floor almost regardless of realistic
admission churn** - this matches what was observed live (24-47m on
adyakonov-cloud, a quiet dev cluster). Don't expect lowering constraint
count or rule cost to move controller-manager's *average* CPU much; it
mostly matters for webhook *tail latency* on individual requests, not
aggregate CPU.

### Caveats on both models

- Calibrated from **one** live measurement (145 pods). Linear scaling with
  pod count is a reasonable assumption (Pod sync dominates), not a verified
  one at large scale - re-run `audit_cycle_cost.sh` on a bigger cluster if
  you can, and update `k` here.
- Both models hold the constraint *set* roughly constant (~64 active
  constraints, as configured on adyakonov-cloud: PSS baseline/restricted +
  2 `SecurityPolicy` CRs). More `SecurityPolicy`/`OperationPolicy` CRs shift
  both models up roughly proportionally to how many additional constraint
  kinds they add.
- These are **average**, not **peak**, CPU figures - an audit cycle is a
  multi-second burst, not a smooth draw; size CPU *limits* (not just
  requests) with headroom above the average, and prefer `vertical-pod-autoscaler`
  over a static guess once you're near or above the numbers in these tables.
