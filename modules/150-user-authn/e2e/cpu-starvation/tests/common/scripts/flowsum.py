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

"""Phase marks and the verdict of a CPU starvation run, executed inside the load pod.

The marks, the flows of dexload.py and the writes of writewatch.py share the clock of the pod.

    flowsum.py mark <NAME>     append "<NAME> <epoch>" to the marks file
    flowsum.py warmup          exit 0 once a login and a refresh have succeeded
    flowsum.py report [opts]   per-phase success rates, latency and writes; exit 1 on a failed check

Phases: baseline [baseline, pressure), pressure [pressure, release), after [release, end).
"""

import argparse
import collections
import sys
import time

p = argparse.ArgumentParser()
p.add_argument("command", choices=["mark", "warmup", "report"])
p.add_argument("name", nargs="?")
p.add_argument("--data", default="/data")
p.add_argument("--recovery", type=float, default=120, help="seconds after the release until logins must work")
p.add_argument("--window", type=float, default=60, help="length of the recovery check window, seconds")
p.add_argument("--min-recovered", type=float, default=0.95, help="login success ratio in the recovery window")
p.add_argument("--min-baseline", type=float, default=0.95, help="login success ratio before the pressure")
p.add_argument("--min-pressure", type=float, default=0.0, help="login success ratio under the pressure")
p.add_argument("--min-flows", type=int, default=5, help="logins a checked window must contain")
p.add_argument("--write-factor", type=float, default=2.0)
p.add_argument("--write-slack", type=float, default=10.0, help="writes per minute above the baseline")
a = p.parse_args()

PHASES = ["baseline", "pressure", "after"]
BOUNDS = {"baseline": ("baseline", "pressure"), "pressure": ("pressure", "release"), "after": ("release", "end")}
LEASES = "leases"

if a.command == "mark":
    if not a.name:
        sys.exit("mark needs a name")
    now = int(time.time())
    with open(f"{a.data}/marks", "a") as f:
        f.write(f"{a.name} {now}\n")
    print(now)
    sys.exit(0)


def read(name, tag):
    rows = []
    try:
        with open(f"{a.data}/{name}") as f:
            for line in f:
                parts = line.strip().split(maxsplit=5)
                if parts and parts[0] == tag:
                    rows.append(parts[1:])
    except FileNotFoundError:
        pass
    return rows


# start_ms, duration_ms, kind, ok, detail
flows = [(int(r[0]) / 1000, int(r[1]), r[2], r[3] == "1", r[4] if len(r) > 4 else "-") for r in read("flows.log", "FLOW")]

if a.command == "warmup":
    kinds = {k for _, _, k, ok, _ in flows if ok}
    print(f"flows={len(flows)} succeeded kinds={sorted(kinds)}")
    sys.exit(0 if {"login", "refresh"} <= kinds else 1)

marks = {}
with open(f"{a.data}/marks") as f:
    for line in f:
        k, v = line.split()
        marks[k] = int(v)
for k in ("baseline", "pressure", "release", "end"):
    if k not in marks:
        sys.exit(f"FAIL: mark {k!r} is missing; marks: {marks}")

writes = [(int(r[0]) / 1000, r[1], r[2], r[3]) for r in read("writes.log", "WRITE")]
failures = []


def phase_of(ts):
    for ph in PHASES:
        lo, hi = BOUNDS[ph]
        if marks[lo] <= ts < marks[hi]:
            return ph
    return None


def minutes(ph):
    lo, hi = BOUNDS[ph]
    return max((marks[hi] - marks[lo]) / 60, 1 / 60)


def pct(sorted_values, q):
    return sorted_values[min(len(sorted_values) - 1, int(len(sorted_values) * q))] if sorted_values else None


def ratio(rows):
    return sum(1 for r in rows if r[3]) / len(rows) if rows else 0.0


by_phase = collections.defaultdict(list)
for row in flows:
    ph = phase_of(row[0])
    if ph:
        by_phase[ph].append(row)

print("Phase lengths: " + ", ".join(f"{ph}={minutes(ph):.1f}m" for ph in PHASES))
print(f"{'phase':<9} {'kind':<8} {'flows':>6} {'rate/s':>7} {'ok%':>6} {'p50ms':>7} {'p95ms':>7} {'maxms':>7}")
for ph in PHASES:
    for kind in ("login", "refresh"):
        rows = [r for r in by_phase[ph] if r[2] == kind]
        if not rows:
            print(f"{ph:<9} {kind:<8} {0:>6}")
            continue
        ok = sorted(r[1] for r in rows if r[3])
        print(f"{ph:<9} {kind:<8} {len(rows):>6} {len(rows) / (minutes(ph) * 60):>7.2f} {100 * ratio(rows):>6.1f} "
              f"{pct(ok, .5) or '-':>7} {pct(ok, .95) or '-':>7} {ok[-1] if ok else '-':>7}")
        errors = collections.Counter(r[4][:80] for r in rows if not r[3])
        for err, n in errors.most_common(3):
            print(f"{'':<17} {n} x {err}")

# Function before the pressure: without it the run proves nothing.
logins = [r for r in by_phase["baseline"] if r[2] == "login"]
if len(logins) < a.min_flows or ratio(logins) < a.min_baseline:
    failures.append(f"baseline: {len(logins)} logins, {100 * ratio(logins):.1f}% ok "
                    f"(need >= {a.min_flows} and >= {100 * a.min_baseline:.0f}%)")

# Function under the pressure.
logins = [r for r in by_phase["pressure"] if r[2] == "login"]
if a.min_pressure > 0 and ratio(logins) < a.min_pressure:
    failures.append(f"pressure: {100 * ratio(logins):.1f}% of {len(logins)} logins ok "
                    f"(need >= {100 * a.min_pressure:.0f}%)")

# Recovery: the logins started in [release + recovery, release + recovery + window) succeed.
lo = marks["release"] + a.recovery
hi = min(lo + a.window, marks["end"])
logins = [r for r in flows if lo <= r[0] < hi and r[2] == "login"]
failed_after = [r[0] for r in flows if marks["release"] <= r[0] < marks["end"] and r[2] == "login" and not r[3]]
last_failure = f"{max(failed_after) - marks['release']:.0f}s" if failed_after else "none"
print(f"Recovery window +{a.recovery:.0f}s..+{hi - marks['release']:.0f}s after the release: "
      f"{len(logins)} logins, {100 * ratio(logins):.1f}% ok; last failed login started {last_failure} after the release")
if len(logins) < a.min_flows or ratio(logins) < a.min_recovered:
    failures.append(f"recovery: {len(logins)} logins, {100 * ratio(logins):.1f}% ok "
                    f"{a.recovery:.0f}s after the release (need >= {a.min_flows} and >= {100 * a.min_recovered:.0f}%)")

# Write storm: MODIFIED events per minute and per completed flow, compared with the baseline.
completed = {ph: sum(1 for r in by_phase[ph] if r[3]) for ph in PHASES}
counts = collections.defaultdict(collections.Counter)
for ts, plural, kind, _ in writes:
    ph = phase_of(ts)
    if ph and kind == "MODIFIED":
        counts[plural][ph] += 1
print(f"Completed flows: " + ", ".join(f"{ph}={completed[ph]}" for ph in PHASES))
print(f"{'MODIFIED':<14} " + " ".join(f"{ph + '/min':>13} {ph + '/flow':>14}" for ph in PHASES))
for plural in sorted(counts):
    c = counts[plural]
    per_min = {ph: c[ph] / minutes(ph) for ph in PHASES}
    per_flow = {ph: (c[ph] / completed[ph] if completed[ph] else float("inf") if c[ph] else 0.0) for ph in PHASES}
    print(f"{plural:<14} " + " ".join(f"{per_min[ph]:>13.1f} {per_flow[ph]:>14.2f}" for ph in PHASES))
    for ph in ("pressure", "after"):
        above_slack = per_min[ph] > per_min["baseline"] + a.write_slack
        if plural == LEASES:
            grows = per_min[ph] > a.write_factor * per_min["baseline"]
            what = f"{per_min[ph]:.1f}/min vs {per_min['baseline']:.1f}/min"
        else:
            # The writes of Dex and the controller follow the logins, so compare them per completed flow.
            grows = per_flow[ph] > a.write_factor * per_flow["baseline"]
            what = (f"{per_flow[ph]:.2f}/flow vs {per_flow['baseline']:.2f}/flow, "
                    f"{per_min[ph]:.1f}/min vs {per_min['baseline']:.1f}/min")
        if above_slack and grows:
            failures.append(f"write storm: {plural} in the {ph} phase: {what} "
                            f"(limit {a.write_factor:g}x the baseline and +{a.write_slack:g}/min)")

for f in failures:
    print(f"FAIL: {f}")
print("RESULT: " + ("FAIL" if failures else "PASS"))
sys.exit(1 if failures else 0)
