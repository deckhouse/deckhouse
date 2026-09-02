# AI Agent Guide: Diagnosing Resource Consumption

You are investigating why `admission-policy-engine` (Gatekeeper) is using
CPU/memory in a Deckhouse cluster - in idle, under load, or after a change.
This file tells you how to do that investigation and what is already known
about this specific module, so you don't have to rediscover it.

## Essential references

| Resource | Path | Purpose |
|----------|------|---------|
| Live diagnostics | [`README.md`](README.md) | What each live script checks, how to read it, the CPU-scaling model, worked examples |
| Broad live sweep | [`live_resource_check.sh`](live_resource_check.sh) | Pods, restarts, usage over time, Gatekeeper's own metrics, cluster-wide scale/events |
| One-cycle cost | [`audit_cycle_cost.sh`](audit_cycle_cost.sh) | Exact CPU-seconds/allocations for one audit cycle, no `--enable-pprof` needed |
| Per-rule time | [`../charts/constraint-templates/tests/tools/bench_rules.py`](../charts/constraint-templates/tests/tools/bench_rules.py) | Isolated Rego eval time per ConstraintTemplate (webhook-representative) |
| Per-rule time+memory | [`../charts/constraint-templates/tests/tools/rulebench.sh`](../charts/constraint-templates/tests/tools/rulebench.sh) | Same, plus `B/op`/`allocs/op` via OPA's Go SDK |
| Test-writing guide | [`../charts/constraint-templates/tests/AGENTS.md`](../charts/constraint-templates/tests/AGENTS.md) | How to add tests/fixtures a new ConstraintTemplate needs before it can be benchmarked |

## Workflow

### Step 1: Static read (no cluster needed)

1. Read the Deployment manifests
   (`../templates/gatekeeper-deployment.yaml`, `../templates/audit-deployment.yaml`):
   container args (especially `--audit-interval`, `--audit-from-cache`,
   `--audit-match-kind-only`), `requests`/`limits`, and the `VerticalPodAutoscaler`
   `resourcePolicy` if present (its `maxAllowed` is the reference ceiling for
   any static `limits` you add - don't invent numbers).
2. Read `../templates/config.yaml` (the Gatekeeper `Config` CR): what gets
   synced into `data.inventory` (cluster-wide, no namespace scoping - a
   Gatekeeper API limitation, not something this chart can fix).
3. Read `../hooks/*.go`: what gets created **unconditionally** at install
   time vs. only when a `SecurityPolicy`/`OperationPolicy` CR exists. This
   distinction matters - see "Known findings" below for why a Pod-sync
   "make it conditional" idea specifically does NOT work here.
4. Count what's always compiled/loaded regardless of configuration (39
   built-in `ConstraintTemplate`s under `../charts/constraint-templates/templates/`).

### Step 2: Live cluster (if you have `kubectl` access)

Run, in order:

```bash
cd modules/015-admission-policy-engine/tools
./live_resource_check.sh                # broad sweep, ~1 minute
./audit_cycle_cost.sh                    # exact cost of one audit cycle
```

Read `README.md` for what each section/number means. Do not eyeball raw
`kubectl top`/`describe`/`events` output by hand when these scripts already
parse and interpret it.

### Step 3: If a specific rule looks implicated

```bash
cd charts/constraint-templates/tests/test_cases/constraints
# generate rendered/ fixtures for the constraint(s) first if not already done -
# see ../../README.md "Quick start"
python3 ../../tools/bench_rules.py . --count 500 --format table --only <name>
../../tools/rulebench.sh . <name>
```

**Read which tool applies to which code path before drawing conclusions**:
`bench_rules.py`/`rulebench` measure one isolated eval - directly
representative of the **webhook** path (one admission review = one object
eval, no discovery/LIST). They are NOT representative of the **audit**
path's total cost - see "Known findings" below. For audit, trust
`audit_cycle_cost.sh`'s live measurement over a per-rule estimate.

### Step 4: Classify every anomaly before touching anything

- **fixable** - controlled by a flag/CR this module owns, safe to change,
  doesn't alter security/correctness behavior.
- **architectural** - inherent to Gatekeeper/OPA or this module's security
  guarantees; cannot be removed without removing the feature. Document why,
  so nobody re-attempts the same "fix" blindly later.
- **environmental** - correlates with node/cluster-wide state (disk
  pressure, CPU steal), not with this module's logic specifically.

### Step 5: Patch, test, document

- Mirror any new static `limits` from the module's own existing VPA
  `maxAllowed` values - don't invent numbers.
- `go test ./modules/015-admission-policy-engine/template_tests/...` before
  proposing a chart change.
- Update `README.md` in this directory and in
  `../charts/constraint-templates/tests/` if you change a tool's interface
  or discover something that changes how the numbers should be read.

## Known findings (as of this investigation)

Treat these as established facts about this module, not hypotheses to
re-derive - but re-verify with the live scripts if you suspect the module
has changed since, or if the numbers don't match what you observe.

1. **Most of `gatekeeper-audit`'s CPU is NOT Rego evaluation.** Measured via
   `audit_cycle_cost.sh` cross-referenced with `rulebench`: pure eval time
   was under 2% of one audit cycle's actual CPU. The rest is Kubernetes API
   interaction - full discovery (scales with CRD count in the cluster), LIST
   calls, JSON→unstructured unmarshalling, and per-constraint status PATCH
   writes. **Implication**: don't expect per-rule micro-optimization to move
   total audit CPU much; `--audit-interval` and the scope of watched/synced
   kinds are the real levers. See `README.md`'s CPU-scaling model section
   for a projection table by cluster size.

2. **`gatekeeper-controller-manager` (webhook) CPU is dominated by a fixed
   ~25m watch/reconcile/leader-election floor**, largely independent of
   admission-request rate at realistic churn levels - here `rulebench`
   numbers ARE directly representative (unlike for audit), since a webhook
   call is exactly the single-object eval that tool measures.

3. **`D8DenyExecHeritage` forces full-cluster `Pod` sync unconditionally.**
   It's created regardless of any `SecurityPolicy` CR (see
   `../templates/policies/security-policy/constraint.yaml`,
   `deny_exec_heritage` block), and Gatekeeper's `Config` sync has no
   per-namespace scoping. **A "sync Pod only when needed" idea was
   investigated and rejected** - there's no configuration under which it
   would actually be conditional, and the risk (silently breaking the
   exec/attach heritage check) isn't worth it. Don't re-propose this without
   new information.

4. **The top-cost rules by both time and memory are the same set**,
   independently confirmed by `bench_rules.py` (CLI timing), `rulebench`
   (Go SDK time+allocs), and a live audit-cycle allocation-count
   cross-check: `allowed-proc-mount`, `read-only-root-filesystem`,
   `allow-privilege-escalation`, `allow-privileged`, `allowed-host-paths`,
   `allowed-cluster-roles`, `allowed-users`. Common cause: per-container SPE
   (SecurityPolicyException) resolution builds and holds intermediate
   objects even when no exception applies. Memory spread across rules
   (~8x) is much narrower than time spread (~100x) - most rules share a
   common per-eval allocation floor from OPA itself (~12,000 B/op, ~230
   allocs/op) before any rule-specific logic runs.

5. **`--audit-interval` is currently `60`** in
   `../templates/audit-deployment.yaml` (upstream Gatekeeper's own default is
   `300`) - this is a proposed, **not yet applied**, fix: lowering it towards
   the upstream default is a straightforward, high-leverage change (worth
   almost exactly the same ~4.3x reduction in average audit CPU at any
   cluster size, per the projection in `README.md`), but no open PR sets it
   as of this writing. Check the manifest yourself before assuming it's
   done, and update this note once it lands.

6. **`automount-service-account-token`'s test fixtures don't build** in
   `rulebench`/`bench_rules.py` - `data.lib.exclude_update.is_update` isn't
   resolvable from the rendered test artifacts. This is a gap in the test
   fixture generation, not in the production ConstraintTemplate. If you fix
   it, remove this note.

## Tooling notes worth knowing before you re-derive them

- `rulebench` has its own `go.mod` (depends on OPA's Go SDK without
  touching the repo's root `go.mod`) - `go run` on a relative path to it
  fails from outside that module; always go through
  [`rulebench.sh`](../charts/constraint-templates/tests/tools/rulebench.sh).
- When polling Gatekeeper's audit log for a specific event (e.g. waiting
  for `audit_finished`), use a generous `--tail` (500+). Right after
  `audit_finished`, Gatekeeper immediately logs one burst line per
  constraint kind - a small tail window can scroll the marker itself out of
  view before your next poll. This caused a real, confusing timeout during
  this investigation (see git history of `audit_cycle_cost.sh`).
- A backgrounded `kubectl port-forward &` **must** redirect its stdin
  (`</dev/null`) if it's launched inside a `while read` loop fed by a pipe -
  otherwise it can silently steal bytes meant for `read` and drop
  iterations. (`live_resource_check.sh` section 4 does this correctly; copy
  the pattern if you add a similar per-pod loop.)
- `date -r <epoch>` means "format an epoch timestamp" on BSD/macOS and "show
  a FILE's mtime" on GNU/Linux (errors on a bare number) - not portable. Use
  `date -u -d "@$epoch" +%T 2>/dev/null || date -u -r "$epoch" +%T` to work
  on both.
- Restarts with `exitCode=0`/`reason=Completed` mean a failed liveness/
  readiness probe killed the container - it did not crash on its own. On a
  shared/dev cluster, correlate with `kubectl get events -A --field-selector
  type=Warning` across a wide timestamp window before blaming this module;
  a node-wide disk-pressure/image-GC problem produces exactly this symptom
  across many unrelated components at once (`live_resource_check.sh`
  section 7 automates this check).
