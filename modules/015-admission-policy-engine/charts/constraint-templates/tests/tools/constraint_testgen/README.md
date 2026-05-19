# `constraint_testgen`

`constraint_testgen` is a utility for maintaining Gatekeeper test suites for Admission Policy Engine constraints.

Current functionality:

- Generate rendered suite artifacts from `ConstraintTestMatrix`.
- Verify `ConstraintTestProfile` contracts against generated suites.
- Validate optional `test_fields.yaml` files during verification.
- Build coverage reports by tracks, case patterns, and field/scenario model.

---

## Supported commands

- `generate -bundle <test-matrix.yaml> [-tests-root <path>]`
- `verify [-tests-root <path>]`
- `coverage [-tests-root <path>] [-constraint <name|path>] [-format table|json|markdown]`

Removed / unsupported in this version:

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

`-tests-root` is optional. If omitted, the tool attempts to auto-detect tests root from current working directory.

---

## `generate -bundle`

Generates rendered artifacts from a single `ConstraintTestMatrix` file.

Input kind is strict: only `ConstraintTestMatrix` is accepted.

Example:

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen generate \
  -bundle ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security/allow-host-processes/test-matrix.yaml \
  -tests-root ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security
```

### What gets generated

Under `spec.outputTestDirectory` (usually `rendered`):

- `test_suite.yaml`
- `constraint-template.yaml`
- `constraints/*.yaml` (copied/resolved as needed)
- `test_samples/...` generated fixtures

### Matrix features currently supported

- Block name mapping via `gatorBlock` (fallback to `name`).
- Legacy `cases: { name, items }` block form is accepted.
- Reusable bases (`spec.bases`) and deep merge for `object`/`inventory` snippets.
- Array semantics in merge patches: arrays replace full values at path.
- Targeted container patching via `containerMerges` and `initContainerMerges`.
- Named reusable exception snippets via `spec.namedExceptions` and case `exception`/`exceptionRef`.
- Optional default/per-case `externalData.providers` support (materialized into generated inventory docs).
- `$TEST_ROOT/...` token support in `ref` values with stable relative normalization in rendered suite paths.
- Automatic cleanup of generated `rendered/test_samples` files that are no longer referenced.
- Constraint template rendering from Helm chart by constraint kind with optional local override from `constraint-template.gator.yaml`.

### Important constraints

- `spec.suiteName` must be a valid RFC1123 subdomain.
- Names explicitly provided in matrix object fragments are validated strictly (no automatic fixing).
- Relative/normalized file paths are enforced to avoid escaping rendered output directory.

---

## `verify`

Verifies each `ConstraintTestProfile` (`test_profile.yaml`) under selected root against generated `rendered/test_suite.yaml`.

Checks include:

- `spec.testDirectory` is set.
- `spec.suite.expectedTestBlockNames` is not empty.
- All expected block names exist in suite `tests[].name`.
- Optional profile coverage policy:
  - `minimumCasesPerBlock`;
  - `requiredPatterns` by track.
- `test_fields.yaml` validation where present:
  - file has valid `ConstraintTestFields` structure;
  - scenario/level values are valid;
  - `metadata.name` equals test directory name (`spec.testDirectory` basename).

Example:

```bash
go run ./modules/015-admission-policy-engine/charts/constraint-templates/tests/tools/constraint_testgen verify \
  -tests-root ./modules/015-admission-policy-engine/charts/constraint-templates/tests/test_cases/constraints/security
```

---

## `coverage`

Builds coverage report across discovered suites (`rendered/test_suite.yaml`).

Coverage includes:

- Per-constraint test counts by track:
  - `functional`
  - `securityPolicyExceptionPod`
  - `securityPolicyExceptionContainer`
- Case pattern buckets (for example: `allowed`, `disallowed`, `allowed-by-exception`, etc.).
- Optional field/scenario coverage (when `test_fields.yaml` and matrix annotations are present):
  - object fields coverage;
  - SPE fields coverage;
  - total required scenarios, covered scenarios, and percentage;
  - list of missing scenarios.
- Optional profile-policy warnings (same policy model as in `verify`).

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
  -format markdown
```

Formats:

- `table` (default)
- `json`
- `markdown` (alias: `md`)

---

## Notes on roots and layouts

The tool supports current per-constraint layout and can resolve roots in different launch contexts:

- direct constraints root;
- parent directories that contain `constraints/...`;
- layouts containing `profiles/`.

For predictable CI behavior, pass explicit `-tests-root`.
