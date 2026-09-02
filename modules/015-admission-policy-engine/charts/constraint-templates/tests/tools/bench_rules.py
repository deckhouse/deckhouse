#!/usr/bin/env python3
#
# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Benchmark the per-request evaluation cost of every Gatekeeper ConstraintTemplate.

Why: Gatekeeper's own Prometheus metrics (gatekeeper_audit_duration_seconds,
gatekeeper_validation_request_duration_seconds) are aggregates for a whole audit
run / webhook call across ALL active constraints - they cannot tell you which
individual rule is expensive. This script isolates each ConstraintTemplate's
compiled Rego and times it directly with `opa eval --count N --metrics`,
using the constraint test suite's own rendered fixtures
(charts/constraint-templates/tests/test_cases/constraints/**/rendered) as
representative inputs - the same objects already used to verify correctness
with `gator verify`.

Reported per constraint:
  - eval_median_us  : cost of ONE `data.<pkg>.violation` evaluation
                       (timer_rego_query_eval_ns, median over --count runs).
                       This is what runs on every webhook admission request
                       AND on every audit-cycle re-check of every matching
                       object in the cluster - the real steady-state/idle cost.
  - compile_ms      : one-off cost of compiling+parsing the module
                       (timer_rego_module_compile_ns + timer_rego_module_parse_ns).
                       Paid once per constraint-template ingestion; maps to
                       Gatekeeper's own gatekeeper_constraint_template_ingestion_duration_seconds.

Two samples are measured per constraint when available - "allowed" (no
violation) and "disallowed" (violation) - picked from the constraint's own
rendered/test_samples/ by filename convention. Both code paths are reported
because some rules do materially more work on one branch than the other.

IMPORTANT - what this number is (and isn't) representative of: one
evaluation here is exactly what the WEBHOOK does for one admission review
(one object, one constraint, no cluster listing). It is NOT representative
of the AUDIT path's total cost: on a live cluster (measured with
../../../../tools/audit_cycle_cost.sh), pure Rego-eval time was under 2%
of one audit cycle's actual CPU - the rest is API discovery (scales with
CRD count), LIST calls, JSON unmarshalling, and per-constraint status PATCH
writes, none of which this script exercises. Use these numbers to compare
rules against each other and to reason about webhook/admission-request
latency; use audit_cycle_cost.sh for the real audit-loop cost.

Prerequisites: `opa` (see repo root Makefile for the pinned OPA_VERSION),
Python 3 with PyYAML, and already-generated `rendered/` fixtures for the
constraints you want to benchmark. From a constraint directory, or in bulk
for all constraints:

    go run ../../tools/constraint_testgen generate -bundle ./test-matrix.yaml
    # or use `make test constraint -- --name <name>` from the chart root,
    # see ../../../README.md and ../../docs/TESTING_GUIDE.md

Usage:
    python3 bench_rules.py <constraints_root> [--count N] [--format table|json] [--only NAME] [--jobs N]

    --jobs/-j runs constraints concurrently (default: min(4, cpu_count)) -
    each `opa eval` subprocess is single-threaded and spends most of its
    time waiting on the child process, so this cuts wall-clock time by
    roughly the job count on a full 39-constraint run. Pass -j1 if you want
    zero cross-benchmark CPU contention (slower, marginally more precise).

Example (from charts/constraint-templates/tests/test_cases/constraints,
after generating rendered/ for the constraints you care about):

    python3 ../../tools/bench_rules.py . --count 500 --format table
"""
import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

import yaml


def extract_rego(rendered_dir: Path):
    ct_path = rendered_dir / "constraint-template.yaml"
    if not ct_path.exists():
        return None
    with open(ct_path) as f:
        ct = yaml.safe_load(f)
    targets = ct.get("spec", {}).get("targets", [])
    if not targets:
        return None
    code = targets[0].get("code")
    if code:
        src = code[0]["source"]
        libs = src.get("libs", [])
        rego = src["rego"]
        v0_compatible = src.get("version") != "v1"
    else:
        # Legacy form: spec.targets[0].rego directly, old Rego syntax
        # (no `if`/`contains` keywords) -> needs --v0-compatible on OPA 1.x.
        libs = []
        rego = targets[0]["rego"]
        v0_compatible = True
    m = re.search(r"^package\s+(\S+)", rego, re.M)
    pkg = m.group(1) if m else None
    return libs, rego, pkg, ct["metadata"]["name"], v0_compatible


def find_samples(rendered_dir: Path):
    """Best-effort (allowed_path, disallowed_path) pair by filename convention.

    Constraint test fixtures are generated by constraint_testgen from
    test-matrix.yaml case names such as "003-006-disallowed-privileged-true.yaml"
    - see ../../docs/TESTING_GUIDE.md section 5. This is a heuristic, not a
    guarantee: for constraints whose case names don't follow the convention,
    the fallback just takes the first two distinct sample files, which may
    both be non-representative. Treat single-run absolute numbers as
    approximate; the relative ranking across constraints is the useful part.
    """
    samples_dir = rendered_dir / "test_samples"
    if not samples_dir.exists():
        return None, None
    allowed, disallowed = None, None
    all_samples = sorted(samples_dir.rglob("*.yaml"))
    for p in all_samples:
        name = p.name.lower()
        if disallowed is None and any(k in name for k in ("disallowed", "violat", "forbidden", "denied")):
            disallowed = p
        elif allowed is None and any(k in name for k in ("allowed", "compliant")):
            allowed = p
        if allowed and disallowed:
            break
    if allowed is None and all_samples:
        allowed = all_samples[0]
    if disallowed is None and len(all_samples) > 1:
        disallowed = all_samples[1]
    return allowed, disallowed


def run_opa_eval(workdir: Path, rego_files, pkg, input_obj, count, v0_compatible):
    input_path = workdir / "input.json"
    with open(input_path, "w") as f:
        json.dump({"review": {"object": input_obj, "operation": "CREATE"}}, f)

    args = ["opa", "eval", "--format=json", f"--count={count}", "--metrics"]
    if v0_compatible:
        args.append("--v0-compatible")
    for rf in rego_files:
        args += ["-d", str(rf)]
    args += ["-i", str(input_path), f"data.{pkg}.violation"]

    proc = subprocess.run(args, capture_output=True, text=True, timeout=120)
    try:
        out = json.loads(proc.stdout)
    except json.JSONDecodeError:
        return None, (proc.stderr.strip() or proc.stdout.strip())[:300] or f"exit={proc.returncode}, no output"
    if proc.returncode != 0 or "errors" in out:
        errs = out.get("errors")
        msg = "; ".join(e.get("message", str(e)) for e in errs) if errs else proc.stderr.strip()
        return None, (msg or f"exit={proc.returncode}")[:300]
    agg = out.get("aggregated_metrics")
    if agg is None:
        return None, "no aggregated_metrics (--count must be > 1)"
    return agg, None


def bench_one(name, group, con_dir: Path, count):
    rendered = con_dir / "rendered"
    extracted = extract_rego(rendered)
    if not extracted:
        return {"name": name, "group": group, "error": "no rendered/constraint-template.yaml (run constraint_testgen generate first)"}
    libs, rego, pkg, ct_name, v0_compatible = extracted
    if not pkg:
        return {"name": name, "group": group, "error": "package not found in rego"}

    allowed_path, disallowed_path = find_samples(rendered)
    if not allowed_path and not disallowed_path:
        return {"name": name, "group": group, "error": "no test samples found under rendered/test_samples"}

    with tempfile.TemporaryDirectory() as tmp:
        tmp = Path(tmp)
        rego_files = []
        for i, lib in enumerate(libs):
            p = tmp / f"lib_{i}.rego"
            p.write_text(lib)
            rego_files.append(p)
        policy_p = tmp / "policy.rego"
        policy_p.write_text(rego)
        rego_files.append(policy_p)

        result = {"name": name, "group": group, "template_kind": ct_name, "package": pkg}
        for label, sample_path in (("allowed", allowed_path), ("disallowed", disallowed_path)):
            if sample_path is None:
                continue
            try:
                obj = yaml.safe_load(sample_path.read_text())
            except Exception as e:
                result[f"{label}_error"] = f"yaml parse: {e}"
                continue
            agg, err = run_opa_eval(tmp, rego_files, pkg, obj, count, v0_compatible)
            if agg is None:
                result[f"{label}_error"] = err or "unknown error"
                continue
            eval_m = agg.get("timer_rego_query_eval_ns", {})
            compile_m = agg.get("timer_rego_module_compile_ns", {})
            parse_m = agg.get("timer_rego_module_parse_ns", {})
            result[f"{label}_eval_median_ns"] = eval_m.get("median")
            result[f"{label}_eval_mean_ns"] = eval_m.get("mean")
            result[f"{label}_eval_p99_ns"] = eval_m.get("99%")
            result[f"{label}_compile_median_ns"] = (compile_m.get("median", 0) or 0) + (parse_m.get("median", 0) or 0)
        return result


def print_table(results):
    rows = []
    for r in results:
        if "error" in r:
            rows.append((f"{r['group']}/{r['name']}", None, None, None, r["error"], None))
            continue
        a = r.get("allowed_eval_median_ns")
        d = r.get("disallowed_eval_median_ns")
        c = r.get("allowed_compile_median_ns") or r.get("disallowed_compile_median_ns")
        vals = [v for v in (a, d) if v is not None]
        worst = max(vals) if vals else None
        err = r.get("allowed_error") or r.get("disallowed_error")
        rows.append((f"{r['group']}/{r['name']}", a, d, c, err, worst))

    rows.sort(key=lambda x: (x[5] is None, -(x[5] or 0)))

    print(f"{'constraint':40s} {'eval allowed (us)':>18s} {'eval violation (us)':>20s} {'compile (ms)':>13s}  note")
    print("-" * 110)
    for name, a, d, c, err, _worst in rows:
        a_s = f"{a/1000:.1f}" if a else ("ERR" if err else "-")
        d_s = f"{d/1000:.1f}" if d else ("ERR" if err else "-")
        c_s = f"{c/1e6:.2f}" if c else "-"
        note = err or ""
        print(f"{name:40s} {a_s:>18s} {d_s:>20s} {c_s:>13s}  {note}")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("constraints_root", help="tests/test_cases/constraints directory (contains operation/ and security/)")
    ap.add_argument("--count", type=int, default=300, help="opa eval --count: number of repeated evaluations per sample (default 300)")
    ap.add_argument("--format", choices=["table", "json"], default="table")
    ap.add_argument("--only", help="only benchmark constraints whose directory name contains this substring")
    ap.add_argument("--jobs", "-j", type=int, default=min(4, os.cpu_count() or 1),
                     help="constraints to benchmark concurrently (default: min(4, cpu_count)). "
                          "Each opa eval subprocess is single-threaded and I/O-bound waiting on "
                          "its child, so this helps a lot on a 39-constraint run; use -j1 for "
                          "maximum measurement precision (no CPU contention between benchmarks) "
                          "at the cost of a ~4x longer run.")
    args = ap.parse_args()

    root = Path(args.constraints_root)
    todo = []
    for group_dir in sorted(root.iterdir()):
        if not group_dir.is_dir() or group_dir.name.startswith(("_", ".")):
            continue
        for con_dir in sorted(group_dir.iterdir()):
            if not con_dir.is_dir() or con_dir.name.startswith("."):
                continue
            if args.only and args.only not in con_dir.name:
                continue
            if not (con_dir / "rendered").exists():
                continue
            todo.append((con_dir.name, group_dir.name, con_dir))

    results = [None] * len(todo)
    with ThreadPoolExecutor(max_workers=max(1, args.jobs)) as pool:
        future_to_idx = {
            pool.submit(bench_one, name, group, con_dir, args.count): i
            for i, (name, group, con_dir) in enumerate(todo)
        }
        for future in as_completed(future_to_idx):
            i = future_to_idx[future]
            name, group, _ = todo[i]
            try:
                results[i] = future.result()
                sys.stderr.write(f"benchmarked {group}/{name}\n")
            except Exception as e:
                # A single `opa` timeout/crash or a missing binary must not
                # discard every other constraint's already-collected results -
                # record it as an error row (same shape bench_one itself
                # uses) and keep going.
                sys.stderr.write(f"benchmarked {group}/{name}: FAILED ({e})\n")
                results[i] = {"name": name, "group": group, "error": f"{type(e).__name__}: {e}"}

    if not results:
        sys.stderr.write("no constraints with rendered/ found - run constraint_testgen generate first (see ../../README.md)\n")
        sys.exit(1)

    if args.format == "json":
        print(json.dumps(results, indent=2))
    else:
        print_table(results)


if __name__ == "__main__":
    main()
