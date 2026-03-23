# constraint_testgen

`constraint_testgen` is a utility for maintaining Gatekeeper test suites for Admission Policy Engine constraints.

Supported commands:

- `generate -bundle` — generate `rendered/test_suite.yaml` and generated fixtures from `ConstraintTestMatrix`.
- `verify` — verify `test_profile.yaml` against generated suites.
- `coverage` — print test and field/scenario coverage.

Removed and unsupported in this version:

- `generate -all`
- `sync-libs`
- `convert-bundle`
- `migrate-refs`
- `ConstraintTestGenerateBundle` input kind

---

## CLI usage

From repository root:

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen <command> [flags]
```

Built-in usage:

```text
verify [-tests-root <path>]
generate -bundle <test-matrix.yaml> [-tests-root <path>]
coverage [-tests-root <path>] [-constraint <name|path>] [-format table|json|markdown]
```

---

## `generate -bundle`

Generates artifacts from a single `ConstraintTestMatrix` file.

Only matrix format is supported. `ConstraintTestGenerateBundle` is no longer accepted.

Example:

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen generate \
  -bundle ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security/allow-host-processes/test-matrix.yaml \
  -tests-root ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security
```

Output includes:

- `<outputTestDirectory>/test_suite.yaml`
- `<outputTestDirectory>/constraint-template.yaml`
- `<outputTestDirectory>/constraints/*.yaml` (if needed)
- `<outputTestDirectory>/test_samples/...` generated fixtures

---

## `verify`

Verifies each `ConstraintTestProfile` (`test_profile.yaml`) against generated `rendered/test_suite.yaml`.

Checks include:

- `spec.testDirectory` is set;
- `spec.suite.expectedTestBlockNames` is not empty;
- all expected block names exist in suite `tests[].name`;
- optional profile coverage policy:
  - `minimumCasesPerBlock`;
  - `requiredPatterns` by track;
- `test_fields.yaml` validation where present.

Example:

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen verify \
  -tests-root ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security
```

---

## `coverage`

Builds coverage report across discovered suites (`rendered/test_suite.yaml`).

Examples:

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen coverage \
  -tests-root ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security \
  -format table
```

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen coverage \
  -tests-root ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security \
  -constraint allow-host-processes \
  -format table
```

Formats:

- `table` (default)
- `json`
- `markdown`
