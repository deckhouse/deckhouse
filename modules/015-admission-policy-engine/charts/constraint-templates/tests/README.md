# Constraint Templates — Tests

This directory contains the testing infrastructure for Gatekeeper ConstraintTemplate policies in Deckhouse.

## Documentation / Документация

| Document | Description |
|----------|-------------|
| [docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md) | **Comprehensive testing guide (EN)** — everything a newcomer needs to write tests from scratch |
| [docs/TESTING_GUIDE.ru.md](docs/TESTING_GUIDE.ru.md) | **Полное руководство по тестированию (RU)** — всё, что нужно новичку для написания тестов с нуля |
| [AGENTS.md](AGENTS.md) | AI agent prompt for writing constraint tests |

## OpenAPI Schemas

Validation schemas for all test YAML files:

- [`openapi/constraint-test-fields.schema.yaml`](openapi/constraint-test-fields.schema.yaml) — `test_fields.yaml` schema
- [`openapi/constraint-test-matrix.schema.yaml`](openapi/constraint-test-matrix.schema.yaml) — `test-matrix.yaml` schema
- [`openapi/constraint-test-profile.schema.yaml`](openapi/constraint-test-profile.schema.yaml) — `test_profile.yaml` schema

## Directory structure

```
tests/
├── docs/                     # Human documentation
├── openapi/                  # JSON Schema validation files
├── tools/constraint_testgen/ # Go code generator tool
├── test_cases/
│   ├── run_all_tests.sh      # Master test runner
│   └── constraints/
│       ├── security/         # Security policy constraints
│       └── operation/        # Operational policy constraints
├── README.md                 # This file
└── AGENTS.md                 # AI agent prompt
```

## Quick start

### Prerequisites

```bash
# Install gator
go install github.com/open-policy-agent/gatekeeper/v3/cmd/gator@latest

# Ensure opa is available
# https://www.openpolicyagent.org/docs/latest/#running-opa
```

### Run all tests

```bash
./test_cases/run_all_tests.sh
```

### Work with a single constraint

From the module root (`modules/015-admission-policy-engine`):

```bash
# Set tool path
constraint_testgen=./tools/constraint_testgen

# Generate rendered artifacts from matrix
go run $constraint_testgen generate \
  -bundle ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>/test-matrix.yaml

# Verify with gator
cd ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>
gator verify -v ./rendered

# Run profile verification
go run $constraint_testgen verify \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints

# Check coverage
go run $constraint_testgen coverage \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints -format table
```

### From within a constraint directory

```bash
constraint_testgen=../../../../tools/constraint_testgen

go run $constraint_testgen generate -bundle ./test-matrix.yaml
gator verify -v ./rendered
go run $constraint_testgen coverage -tests-root ./ -format table
```

For detailed instructions, see [docs/TESTING_GUIDE.md](docs/TESTING_GUIDE.md).
