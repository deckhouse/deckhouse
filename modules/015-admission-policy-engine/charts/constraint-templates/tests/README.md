# Constraint Templates — Tests

This directory contains the testing infrastructure for Gatekeeper ConstraintTemplate policies in Deckhouse.

## Documentation

| Document                                             | Description                                                                                    |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| [docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md)       | **Comprehensive testing guide (EN)** — everything a newcomer needs to write tests from scratch |
| [docs/TESTING_GUIDE_RU.md](docs/TESTING_GUIDE_RU.md) | **Comprehensive testing guide (RU)** — everything a newcomer needs to write tests from scratch |
| [AGENTS.md](AGENTS.md)                               | AI agent prompt for writing constraint tests                                                   |

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
├── tools/constraint_testgen/ # Go code generator tool
├── test_cases/
│   ├── run_all_tests.sh      # Master test runner
│   ├── libs/                 # Library with test files (simlink to templates/libs folder)
│   └── constraints/
│       ├── security/         # Security policy constraints
│       └── operation/        # Operational policy constraints
├── README.md                 # This file
└── AGENTS.md                 # AI agent prompt
```

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

`--docker` mode builds [`images/test-container/dockerfile`](images/test-container/dockerfile) and runs tests in a container with repository files mounted.

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

