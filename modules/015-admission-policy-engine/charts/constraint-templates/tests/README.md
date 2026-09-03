# Constraint Templates — Tests

This directory contains the testing infrastructure for Gatekeeper ConstraintTemplate policies in Deckhouse.

## Documentation

| Document                                             | Description                                                                                    |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| [docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md)       | **Comprehensive testing guide (EN)** — everything a newcomer needs to write tests from scratch |
| [docs/TESTING_GUIDE_RU.md](docs/TESTING_GUIDE_RU.md) | **Comprehensive testing guide (RU)** — everything a newcomer needs to write tests from scratch |
| [AGENTS.md](AGENTS.md)                               | AI agent prompt for writing constraint tests                                                   |
| [tools/bench_rules.py](tools/bench_rules.py)         | Per-constraint Rego evaluation **time** benchmark (which rule is slow)                        |
| [tools/rulebench.sh](tools/rulebench.sh)             | Per-constraint Rego evaluation **time + memory** benchmark (which rule allocates)              |

## How tests are organized

### OpenAPI schemas

Validation schemas for all test YAML files:

- [`openapi/constraint-test-fields.schema.yaml`](openapi/constraint-test-fields.schema.yaml) — `test_fields.yaml` schema
- [`openapi/constraint-test-matrix.schema.yaml`](openapi/constraint-test-matrix.schema.yaml) — `test-matrix.yaml` schema
- [`openapi/constraint-test-profile.schema.yaml`](openapi/constraint-test-profile.schema.yaml) — `test_profile.yaml` schema

### Directory structure

```
tests/
├── docs/                     # Human documentation
├── openapi/                  # JSON Schema validation files
├── tools/
│   ├── constraint_testgen/   # Go code generator tool
│   ├── bench_rules.py        # Per-rule evaluation time benchmark (opa eval CLI)
│   ├── rulebench.sh          # Wrapper for rulebench/ (works around its module boundary)
│   └── rulebench/            # Per-rule evaluation time + memory benchmark (OPA Go SDK,
│                              # own go.mod/go.sum - isolated from the repo's root module)
├── test_cases/
│   ├── run_all_tests.sh      # Master test runner
│   ├── libs/                 # Library with test files (simlink to templates/libs folder)
│   └── constraints/
│       ├── security/         # Security policy constraints
│       └── operation/        # Operational policy constraints
├── README.md                 # This file
└── AGENTS.md                 # AI agent prompt
```

> Important: do not add extra large files to this directory. In tests, this directory is processed as a Helm chart, and additional heavy files can break template rendering.

### Test flow at a glance

- Define test scenarios in `test_fields.yaml`, `test-matrix.yaml`, and `test_profile.yaml`.
- Generate rendered artifacts with `constraint_testgen`.
- Verify rendered suites with `gator`.
- Check profile and coverage with `constraint_testgen verify|coverage`.

## Quick start

From chart root (`modules/015-admission-policy-engine/charts/constraint-templates`), choose one execution mode:

```bash
# Option 1: Run locally (without Docker)
make test all

# Option 2: Run in Docker
make test all -- --docker
```

For manual run without Makefile, see [Run without Docker (without Makefile)](#run-without-docker-without-makefile).

## Makefile-based test entrypoint

Run commands from chart root [`../Makefile`](../Makefile):

```bash
# Run full test flow (OPA library tests + generate + gator + coverage)
make test all

# Same flow in Docker
make test all -- --docker

# Coverage for all constraints only
make test coverage all
make test coverage all -- --docker

# Generate + gator verify for one constraint
make test constraint -- --name <constraint-name>
make test constraint -- --name <constraint-name> --docker

# Coverage for one constraint
make test coverage constraint -- --name <constraint-name>
make test coverage constraint -- --name <constraint-name> --docker
```

`--docker` mode builds [`images/test-container/Dockerfile`](images/test-container/Dockerfile) and runs tests in a container with repository files mounted.

> GNU Make treats `--docker` and `--name` as options. Use the `--` separator exactly as shown in examples.

Version source note:

- Runtime versions for `opa` and `gator` are defined in repository root [`../../../../../Makefile`](../../../../../Makefile).
- Chart-level [`../Makefile`](../Makefile) reads these values from the root file.

## Run without Docker (without Makefile)

### Prerequisites

Install `opa` and `gator` before running tests directly.

> Required `OPA` and `gator` versions must be taken from repository root [`../../../../../Makefile`](../../../../../Makefile), not from chart-level [`../Makefile`](../Makefile).

```bash
GIT_ROOT=$(git rev-parse --show-toplevel)

# Show required versions from repository root Makefile
awk -F '=' '/^OPA_VERSION[[:space:]]*=/{gsub(/[[:space:]]/,"",$2); print "OPA_VERSION=" $2}' ${GIT_ROOT}/Makefile
awk -F '=' '/^GATOR_VERSION[[:space:]]*=/{gsub(/[[:space:]]/,"",$2); print "GATOR_VERSION=" $2}' ${GIT_ROOT}/Makefile

# Install gator (example: replace version with value from root Makefile)
go install github.com/open-policy-agent/gatekeeper/v3/cmd/gator@v3.22.0

# Install opa (macOS via Homebrew example)
brew install opa
```

```bash
# Check currently installed versions
opa version
gator version
```

### Run full test flow with script (without Makefile)

A ready-to-use script is available at [`test_cases/run_all_tests.sh`](test_cases/run_all_tests.sh).

From the module root (`modules/015-admission-policy-engine`):

```bash
./charts/constraint-templates/tests/test_cases/run_all_tests.sh
```

From the tests directory (`modules/015-admission-policy-engine/charts/constraint-templates/tests`):

```bash
./test_cases/run_all_tests.sh
```

The script runs the full local flow: OPA library tests, generation, `gator` verification, and coverage checks.

### Direct run without Docker and without Makefile

From the module root (`modules/015-admission-policy-engine`):

```bash
# Set tool path
GIT_ROOT=$(git rev-parse --show-toplevel)
CHART_DIR=${GIT_ROOT}/modules/015-admission-policy-engine/charts/constraint-templates
constraint_testgen=${CHART_DIR}/tests/tools/constraint_testgen

# 1) Generate artifacts for one constraint

go run $constraint_testgen generate \
  -bundle ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>/test-matrix.yaml

# 2) Verify rendered suite with gator
cd ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>
gator verify -v ./rendered

# 3) Verify profiles for all constraints
cd ${GIT_ROOT}/modules/015-admission-policy-engine
go run $constraint_testgen verify \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints

# 4) Check coverage for all constraints
go run $constraint_testgen coverage \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints -format table
```

From within a constraint directory:

```bash
GIT_ROOT=$(git rev-parse --show-toplevel)
CHART_DIR=${GIT_ROOT}/modules/015-admission-policy-engine/charts/constraint-templates
constraint_testgen=${CHART_DIR}/tests/tools/constraint_testgen

go run $constraint_testgen generate -bundle ./test-matrix.yaml
gator verify -v ./rendered
go run $constraint_testgen verify
go run $constraint_testgen coverage -tests-root ./ -format table
```

For detailed instructions, see [docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md).

## Measuring per-rule cost (time & memory)

Gatekeeper's own metrics (`gatekeeper_audit_duration_seconds`,
`gatekeeper_validation_request_duration_seconds`) are aggregates across every
active constraint in one audit run / webhook call — they cannot tell you which
individual rule is expensive, and they carry no memory signal at all. Two
tools isolate each ConstraintTemplate's compiled Rego and benchmark it
directly, reusing the constraint's own `rendered/test_samples/` fixtures —
the same objects already used to verify correctness with `gator verify`.

Both share the same caveat: sample selection is filename-convention based
(`*allowed*` / `*disallowed*`, falling back to the first two files) — treat
absolute numbers as approximate, the relative ranking across constraints is
the useful part. See each tool's docstring/doc-comment for details.

**What these numbers are (and aren't) representative of.** Both tools
measure exactly what the **webhook** does for one admission review: one
object, one constraint, no cluster listing. They are directly representative
of webhook/admission-request latency. They are **not** representative of the
**audit** loop's total cost: measured on a live cluster with
[`../../../tools/audit_cycle_cost.sh`](../../../tools/audit_cycle_cost.sh)
(diffs Go-runtime counters across one real audit cycle's exact start/end, no
`--enable-pprof` needed), pure Rego-eval time was under 2% of that cycle's
actual CPU — the rest is API discovery (scales with CRD count in the
cluster), LIST calls, JSON unmarshalling, and per-constraint status PATCH
writes, none of which these two tools exercise. Use `bench_rules.py`/
`rulebench` to compare rules against each other and to reason about the
webhook path; use `audit_cycle_cost.sh` (and
[`../../../tools/live_resource_check.sh`](../../../tools/live_resource_check.sh)
for the broader live picture) for the real audit-loop cost. See
`../../../tools/README.md` for that whole live-cluster side of the
toolkit, with a worked example.

### Time: `tools/bench_rules.py`

Shells out to `opa eval --count N --metrics` for a single
`data.<pkg>.violation` evaluation. No extra dependencies beyond `opa` itself.

```bash
# Generate rendered/ fixtures first (see "Quick start" above), then:
cd modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints
python3 ../../tools/bench_rules.py . --count 500 --format table
python3 ../../tools/bench_rules.py . --count 500 --format table -j 8   # faster, more CPU contention
python3 ../../tools/bench_rules.py . --count 500 --format table -j 1   # slowest, most precise
```

Output is a table sorted by evaluation cost (µs), with a one-off Rego
compile-time column for reference — that maps to Gatekeeper's own
`gatekeeper_constraint_template_ingestion_duration_seconds` and is paid once
per template, not per request. Constraints are benchmarked concurrently
(`-j`/`--jobs`, default `min(4, cpu_count)`) since each `opa eval` subprocess
mostly waits on its child process — measured on a full 39-constraint,
`--count 500` run: ~2m27s sequential (`-j1`) vs ~1m02s at the default job
count. Pass `-j1` if you want zero cross-benchmark CPU contention (ranking
stays the same either way; only the exact µs values get a bit noisier under
concurrency, similar to normal run-to-run jitter).

### Time + memory: `tools/rulebench`

`opa eval`'s metrics are timers only — no bytes-allocated or allocs-per-op
signal. [`tools/rulebench`](tools/rulebench/main.go) is a small standalone Go
tool (own `go.mod`/`go.sum`, so it depends on
`github.com/open-policy-agent/opa` without touching the repo's root
`go.mod`) that builds a `rego.PreparedEvalQuery` once per constraint with
OPA's own Go SDK and drives it with `testing.Benchmark` + `ReportAllocs()`,
reporting `ns/op`, `B/op` (bytes allocated per evaluation), and `allocs/op` —
the real heap pressure a rule puts on Gatekeeper's garbage collector, not
just wall-clock time.

`rulebench` has its own `go.mod`, so plain `go run ../../tools/rulebench .`
fails from outside that module (`main module ... does not contain package
...`) — `go run <relative-path>` resolves in the module of the *current*
directory, and `test_cases/constraints` belongs to the repo's root module,
not to rulebench's. Use the [`tools/rulebench.sh`](tools/rulebench.sh)
wrapper instead, which resolves the constraints-root argument to an absolute
path and `cd`s into rulebench's own module directory before running it:

```bash
# Generate rendered/ fixtures first (see "Quick start" above), then:
cd modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints
../../tools/rulebench.sh .                   # all constraints
../../tools/rulebench.sh . allow-privileged  # one constraint
```

Unlike `bench_rules.py`, this runs constraints **sequentially**, on purpose:
Go's `testing` package serializes the actual timed portion of every
`testing.Benchmark` call process-wide through its own package-level
`benchmarkLock` (`$GOROOT/src/testing/benchmark.go`), so concurrent
goroutines calling it don't run their timed loops in parallel at all — only
their `PrepareForEval`/compile setup work can overlap, and letting that
setup run concurrently with someone else's *timed* window can leak a few of
its allocations into that window's `ReportAllocs()` numbers. Since this
tool's whole point is a trustworthy `B/op`/`allocs/op`, that trade-off isn't
worth chasing a small wall-clock win for - unlike `bench_rules.py`, which
benchmarks via separate `opa` subprocesses with no such shared lock, so its
`-j` concurrency is real.

In practice the two tools agree closely: the rules that are slowest under
`opa eval` are also the ones allocating the most under `rulebench` — both
point at the same root cause (per-container SPE exception resolution), which
is good corroboration from two independent measurement paths. The memory
spread across rules is also notably narrower than the time spread — most
rules share a common per-eval allocation floor from OPA itself, on top of
which SPE-heavy rules add a large multiplier. A third, independent
corroboration exists too: on a live cluster, the observed audit-cycle
allocation count divided by (synced objects × active constraints) landed
almost exactly in this range — see the worked example in
`../../../tools/README.md`.

