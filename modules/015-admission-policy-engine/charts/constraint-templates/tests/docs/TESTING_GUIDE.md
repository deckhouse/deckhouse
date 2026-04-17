# Constraint Templates Testing Guide (EN)

This guide covers everything you need to write, run, and maintain tests for Gatekeeper `ConstraintTemplates` in Deckhouse. It is written for newcomers with **zero prior context**.

> **Validation schemas** for every YAML file described below live in [`../openapi/`](../openapi/). Use them as the authoritative reference for allowed fields and values.

---

## Table of contents

1. [Glossary](#1-glossary)
2. [Directory structure](#2-directory-structure)
3. [Files per constraint](#3-files-per-constraint)
4. [test_fields.yaml — field/scenario model](#4-test_fieldsyaml--fieldscenario-model)
5. [test-matrix.yaml — test cases](#5-test-matrixyaml--test-cases)
6. [test_profile.yaml — suite/quality contract](#6-test_profileyaml--suitequality-contract)
7. [Scenario model](#7-scenario-model)
8. [Coverage calculation](#8-coverage-calculation)
9. [Step-by-step: adding tests to a new constraint](#9-step-by-step-adding-tests-to-a-new-constraint)
10. [Step-by-step: adding tests to an existing constraint](#10-step-by-step-adding-tests-to-an-existing-constraint)
11. [Useful commands](#11-useful-commands)
12. [Known limitations](#12-known-limitations)
13. [Troubleshooting](#13-troubleshooting)
14. [Definition of Done](#14-definition-of-done)

---

## 1. Glossary

| Term                 | Meaning                                                                                                                                                                                           |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Constraint`         | A Gatekeeper policy that validates Kubernetes objects (e.g. Pods). Each constraint has its own test directory.                                                                                    |
| `ConstraintTemplate` | The Rego-based template that defines the policy logic. Lives under `charts/constraint-templates/templates/`.                                                                                      |
| `SPE`                | `SecurityPolicyException` — a Deckhouse CRD that allows exceptions for a constraint. SPE fields are defined in [`security-policy-exception.yaml`](../../../../crds/security-policy-exception.yaml). |
| `Track`              | A group of tests: *Functional*, *SPE Pod*, or *SPE Container*.                                                                                                                                    |
| `Scenario`           | A specific test angle for a field (positive, negative, absent, etc.).                                                                                                                             |
| `Block`              | A named section in the generated test suite (`rendered/test_suite.yaml`), grouping cases that share a template+constraint pair.                                                                   |
| `Gator`              | The OPA Gatekeeper CLI tool used to verify constraint tests offline.                                                                                                                              |
| `constraint_testgen` | The Go-based code generator that converts `test-matrix.yaml` into rendered test artifacts.                                                                                                        |

---

## 2. Directory structure

All test infrastructure lives under:

```shell
charts/constraint-templates/tests/
├── docs/                  # Human documentation (this file)
├── openapi/               # JSON Schema files for validation
│   ├── constraint-test-fields.schema.yaml
│   ├── constraint-test-matrix.schema.yaml
│   └── constraint-test-profile.schema.yaml
├── tools/
│   └── constraint_testgen/   # Go tool: generate, verify, coverage
├── test_cases/
│   ├── run_all_tests.sh      # Master test runner (OPA + gator + coverage)
│   └── constraints/
│       ├── security/          # Security policy constraints
│       │   ├── allow-host-network/
│       │   ├── allow-privilege-escalation/
│       │   ├── allow-privileged/
│       │   └── ...
│       └── operation/         # Operational policy constraints
│           ├── allowed-repos/
│           ├── container-resources/
│           └── ...
├── README.md
└── AGENTS.md              # AI agent prompt
```

Constraint groups: **`security`** and **`operation`**.

---

## 3. Files per constraint

Each constraint directory (e.g. `test_cases/constraints/security/allow-host-network/`) contains:

| File                                | Purpose                                                                                               | Hand-written? |
| ----------------------------------- | ----------------------------------------------------------------------------------------------------- | :-----------: |
| `test_fields.yaml`                  | Declares what fields the policy checks and what scenarios are required. Source of truth for coverage. |       ✅       |
| `test-matrix.yaml`                  | Declares test cases with `fields` annotations linking each case to field+scenario pairs.              |       ✅       |
| `test_profile.yaml`                 | Suite/quality contract: required test block names, optional quality gates.                            |       ✅       |
| `constraints/`                      | Constraint YAML manifests referenced by the matrix (e.g. `pss_baseline.yaml`, `policy_1.yaml`).       |       ✅       |
| `rendered/`                         | **Generated artifacts — do not edit manually.**                                                       |       ❌       |
| `rendered/test_suite.yaml`          | Flattened test plan consumed by gator.                                                                |       ❌       |
| `rendered/test_samples/`            | Generated Pod/object YAML samples for each case.                                                      |       ❌       |
| `rendered/constraint-template.yaml` | Rendered `ConstraintTemplate` from Helm chart.                                                          |       ❌       |
| `rendered/constraints/`             | Rendered constraint copies for gator.                                                                 |       ❌       |

### Constraint file naming conventions

- `pss_baseline.yaml` — baseline-compatible constraint input
- `pss_restricted.yaml` — restricted-compatible constraint input
- `policy_<n>.yaml` — additional scenario-specific constraints (numbering starts from 1)

---

## 4. test_fields.yaml — field/scenario model

### Purpose

This file describes **what exactly the policy checks** and **how many test scenarios each field requires**. It is the **source of truth** for scenario-based field coverage.

### Schema

> Full schema: [`../openapi/constraint-test-fields.schema.yaml`](../openapi/constraint-test-fields.schema.yaml)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: <constraint-name>          # Must match directory name
spec:
  objectKind: Pod                   # Kubernetes kind under test
  objectFields:                     # Fields the Rego rule reads/compares
    - path: spec.hostNetwork
      level: pod                    # pod | container | initContainer
      description: "Whether the pod uses the host network namespace"
      requiredScenarios:            # Omit to use defaults for the level
        - positive
        - negative
        - absent
  speFields:                        # Optional: SPE fields that override the constraint
    - path: spec.network.hostNetwork.allowedValue
      level: pod                    # pod | container
      description: "SPE override for hostNetwork"
      requiredScenarios:
        - speMatch
        - speMismatch
        - speAbsent
  applicableTracks:
    functional: true                # Standard (non-exception) cases exist
    spePod: true                    # SPE at pod level
    speContainer: false             # SPE at container level
```

### Key rules

1. **`metadata.name`** must equal the constraint directory name.
2. **`objectKind`** is the main Kubernetes kind validated (usually `Pod`).
3. **`objectFields`** lists every object field the Rego rule reads or compares.
4. **`speFields`** lists every SPE field the Rego rule reads. Always verify paths against the SPE CRD.
5. **`level`** determines default scenarios:
   - `pod` → fields under `spec.*` belonging to the Pod
   - `container` → fields under `spec.containers[].*`
   - `initContainer` → fields under `spec.initContainers[].*`
6. **`requiredScenarios`** can be omitted to use defaults (see [Scenario model](#7-scenario-model)).
7. **`applicableTracks`** — at least one track must be `true`.

### Real example (allow-host-network)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: allow-host-network
spec:
  objectKind: Pod
  objectFields:
    - path: spec.hostNetwork
      level: pod
      description: Whether the pod uses the host network namespace
      requiredScenarios: [positive, negative, absent]
    - path: spec.containers[].ports[].hostPort
      level: container
      description: Host port mapping for container
      requiredScenarios: [positive, negative, absent, multiContainer, initContainer, ephemeralContainer]
    - path: spec.containers[].ports[].protocol
      level: container
      description: Protocol for host port
      requiredScenarios: [positive, negative, absent, multiContainer, initContainer, ephemeralContainer]
  speFields:
    - path: spec.network.hostNetwork.allowedValue
      level: pod
      description: SPE override for hostNetwork
      requiredScenarios: [speMatch, speMismatch, speAbsent]
    - path: spec.network.hostPorts[].port
      level: pod
      description: SPE allowed host port
      requiredScenarios: [speMatch, speMismatch, speAbsent]
    - path: spec.network.hostPorts[].protocol
      level: pod
      description: SPE allowed host port protocol
      requiredScenarios: [speMatch, speMismatch, speAbsent]
  applicableTracks:
    functional: true
    spePod: true
    speContainer: false
```

---

## 5. test-matrix.yaml — test cases

### Purpose

Defines **test cases** organized into **blocks**. Each case declares which field+scenario it covers via the `fields` array, enabling automated coverage calculation.

### Schema

> Full schema: [`../openapi/constraint-test-matrix.schema.yaml`](../openapi/constraint-test-matrix.schema.yaml)

### Top-level structure

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: <constraint-name>
spec:
  suiteName: d8-<constraint-name>        # Name for rendered Suite
  outputTestDirectory: rendered           # Output directory (always "rendered")
  defaultObjectBase: admissionPod         # Default base key for case objects
  defaultInventory:                       # Inventory prepended to every case
    - ref: ../../../_test-samples/ns.yaml
  bases:                                  # Reusable base documents
    admissionPod:
      document:
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: testns
        spec:
          containers:
            - image: nginx
              name: nginx
    securityPolicyException:
      document:
        apiVersion: deckhouse.io/v1alpha1
        kind: SecurityPolicyException
        metadata:
          namespace: testns
  namedExceptions: {}                     # Reusable exception fragments
  externalData:                           # Optional: external data providers for gator
    providers: []
  blocks:                                 # Test blocks (see below)
    - name: ...
```

### Blocks

Each block maps to a `tests[]` entry in the generated `test_suite.yaml`:

```yaml
blocks:
  - name: pss-baseline-functional         # Human-readable name
    gatorBlock: pss-baseline-functional    # Gator test block name (overrides name)
    template: ../rendered/constraint-template.yaml
    constraint: constraints/pss_baseline.yaml
    cases:
      - name: allowed-no-hostnetwork
        violations: "no"                   # "yes" or "no"
        fields:                            # Coverage annotations
          - path: spec.hostNetwork
            scenario: absent
        object:                            # The object under test
          base: admissionPod
          merge:
            metadata:
              name: allowed-no-hostnetwork
            spec:
              containers:
                - image: nginx
                  name: nginx
                  ports:
                    - containerPort: 80
```

### The `object.merge` pattern

Cases use a **base + merge** pattern:
- `base` references a key from `spec.bases`
- `merge` is a deep-merge patch applied on top of the base document
- Maps recurse; **arrays in `merge` replace** the whole array at that path
- Use `containerMerges` / `initContainerMerges` for single-container deltas when the base already defines that container

### Fields annotations

Each case should declare which field+scenario pairs it covers:

```yaml
fields:
  - path: spec.hostNetwork        # Must exactly match test_fields.yaml path
    scenario: negative             # Must be a valid scenario name
  - path: spec.containers[].ports[].hostPort
    scenario: multiContainer
```

**Rules:**
- Every `path` must **byte-for-byte match** a `path` from `test_fields.yaml`
- Every `scenario` must be a valid scenario name
- Use SPE field paths and SPE scenarios only for exception cases
- A case may cover multiple field+scenario pairs
- Multiple cases may cover the same pair (only one is needed for coverage)

### SPE cases

For `SecurityPolicyException` cases, add the exception as inventory:

```yaml
cases:
  - name: allowed-by-exception-hostnetwork
    violations: "no"
    fields:
      - path: spec.network.hostNetwork.allowedValue
        scenario: speMatch
    inventory:
      - base: securityPolicyException
        merge:
          metadata:
            name: allow-hostnetwork-true
          spec:
            network:
              hostNetwork:
                allowedValue: true
    object:
      base: admissionPod
      merge:
        metadata:
          labels:
            security.deckhouse.io/security-policy-exception: allow-hostnetwork-true
          name: allowed-by-exception-hostnetwork
        spec:
          hostNetwork: true
          containers:
            - image: nginx
              name: nginx
```

Key points for `SPE` cases:
- The Pod must have the label `security.deckhouse.io/security-policy-exception: <exception-name>`
- The exception name in the label must match the SPE `metadata.name`
- `SPE` inventory uses `base: securityPolicyException` with a `merge` patch

### Real example (operation constraint — allowed-repos)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: allowed-repos
spec:
  suiteName: d8-allowed-repos
  outputTestDirectory: rendered
  defaultObjectBase: admissionPod
  defaultInventory:
    - ref: ../../../_test-samples/ns.yaml
  bases:
    admissionPod:
      document:
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: testns
        spec:
          containers:
            - image: nginx
              name: nginx
  blocks:
    - name: operation-policy
      gatorBlock: operation-policy
      template: ../../templates/operation/allowed-repos.yaml
      constraint: constraints/policy_1.yaml
      cases:
        - name: example-allowed
          violations: "no"
          fields:
            - path: spec.containers[].image
              scenario: positive
          object:
            base: admissionPod
            merge:
              metadata:
                name: allowed
              spec:
                containers:
                  - name: foo
                    image: my.repo/app:v1
        - name: example-disallowed
          violations: "yes"
          fields:
            - path: spec.containers[].image
              scenario: negative
          object:
            base: admissionPod
            merge:
              metadata:
                name: disallowed
              spec:
                containers:
                  - name: foo
                    image: gcr.io/app:v1
```

---

## 6. test_profile.yaml — suite/quality contract

### Purpose

Per-constraint verification profile. Declares which test blocks **must** be present in the generated suite and optional quality gates.

### Schema

> Full schema: [`../openapi/constraint-test-profile.schema.yaml`](../openapi/constraint-test-profile.schema.yaml)

### Minimal template

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <constraint-name>
spec:
  testDirectory: <constraint-name>
  suite:
    expectedTestBlockNames:
      - <block-name-1>
      - <block-name-2>
```

### Extended template (optional quality gates)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <constraint-name>
spec:
  testDirectory: <constraint-name>
  suite:
    expectedTestBlockNames:
      - pss-baseline-functional
      - security-policy-functional
  coverage:
    minimumCasesPerBlock: 1
    requiredPatterns:
      functional:
        - "*negative*"
      securityPolicyExceptionPod:
        - "*spe*"
```

### Real example (allow-host-network)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: allow-host-network
spec:
  testDirectory: allow-host-network
  suite:
    expectedTestBlockNames:
      - pss-baseline-functional
      - security-policy-functional
      - security-policy-spe-pod
```

### How to form test_profile.yaml

1. Set `metadata.name` to the constraint directory name.
2. Set `spec.testDirectory` to the same name.
3. Fill `spec.suite.expectedTestBlockNames` with all block names from your `test-matrix.yaml` (the `gatorBlock` or `name` values).
4. Optionally set `spec.coverage.*` for stricter quality gates.

### Separation of responsibilities

| File                | Responsibility                                                                 | Does NOT define                                               |
| ------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------------------- |
| `test_fields.yaml`  | Field/scenario model: object/`SPE` fields, required scenarios, applicable tracks | Suite block names, minimum cases per block, required patterns |
| `test_profile.yaml` | Suite/quality contract: required test blocks, minimum cases, required patterns | Field inventory and per-field scenarios                       |

---

## 7. Scenario model

A **scenario** is a specific test angle for a field. Required scenarios define the **minimum set of angles** that must be covered for a field to be considered fully tested.

### Object-field scenarios (Functional track)

| Scenario             | Meaning                                                 | Applies to           |
| -------------------- | ------------------------------------------------------- | -------------------- |
| `positive`           | Field set to a compliant value → no violation           | All object fields    |
| `negative`           | Field set to a non-compliant value → violation          | All object fields    |
| `absent`             | Field not set at all → depends on `defaultBehavior`     | All object fields    |
| `multiContainer`     | Multiple containers, one violating → violation          | Container-level only |
| `initContainer`      | `initContainer` variant of the check                    | Container-level only |
| `ephemeralContainer` | `ephemeralContainer` variant (`spec.ephemeralContainers`) | Container-level only |

### SPE scenarios (Exception tracks)

| Scenario               | Meaning                                         | Applies to               |
| ---------------------- | ----------------------------------------------- | ------------------------ |
| `speMatch`             | SPE matches the violation → exception allows it | All `SPE` fields           |
| `speMismatch`          | SPE does not match → still violated             | All `SPE` fields           |
| `speAbsent`            | No SPE label on pod → still violated            | All `SPE` fields           |
| `speContainerSpecific` | SPE targets a specific container                | Container-level `SPE` only |

### Default required scenarios by level

**Object fields:**

| Level           | Default required scenarios                                                    | Count |
| --------------- | ----------------------------------------------------------------------------- | ----- |
| `pod`           | positive, negative, absent                                                    | 3     |
| `container`     | positive, negative, absent, multiContainer, initContainer, ephemeralContainer | 6     |
| `initContainer` | positive, negative, absent                                                    | 3     |

**SPE fields:**

| Level       | Default required scenarios                             | Count |
| ----------- | ------------------------------------------------------ | ----- |
| `pod`       | speMatch, speMismatch, speAbsent                       | 3     |
| `container` | speMatch, speMismatch, speAbsent, speContainerSpecific | 4     |

If `requiredScenarios` is omitted in `test_fields.yaml`, the tool auto-generates the default set based on `level`.

---

## 8. Coverage calculation

`constraint_testgen coverage` calculates scenario coverage from `test_fields.yaml` + `test-matrix.yaml`, and reads `test_profile.yaml` for profile-level checks.

### Formula

```shell
Per-field coverage = scenarios covered / scenarios required
Total coverage %   = sum(covered scenarios) / sum(required scenarios) × 100
```

A scenario is "covered" if at least one case in `test-matrix.yaml` declares `fields` with that path+scenario pair.

### How to achieve 100% coverage

1. For every field in `test_fields.yaml`, check its `requiredScenarios` (or defaults).
2. For each required scenario, ensure at least one case in `test-matrix.yaml` has a matching `fields` entry with the exact `path` and `scenario`.
3. Run coverage to verify — any missing scenarios are listed as warnings.

### Example output

```shell
Constraint              Fields  Scenarios  Covered  %     Status
allow-host-network      4+3     25         25       100%  OK
allow-privilege-escal.  1+1     9          6        67%   WARN
  missing: spec.containers[].securityContext.allowPrivilegeEscalation/multiContainer
  missing: spec.containers[].securityContext.allowPrivilegeEscalation/initContainer
```

---

## 9. Step-by-step: adding tests to a new constraint

Use this when creating tests for a brand-new constraint from scratch.

### Step 1: Locate policy inputs

- Read the constraint Rego template and list every object field it reads/compares.
- Read the template parameters and list knobs that affect policy behavior.
- Read `SPE` CRD paths in [`security-policy-exception.yaml`](../../../../crds/security-policy-exception.yaml) and map only paths actually used by this constraint.

### Step 2: Create test_fields.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: <your-constraint>
spec:
  objectKind: Pod
  objectFields:
    - path: <field-path>
      level: pod|container|initContainer
      description: "<what this field does>"
  speFields:                              # Only if SPE is supported
    - path: <spe-field-path>
      level: pod|container
      description: "<what this SPE field does>"
  applicableTracks:
    functional: true
    spePod: true|false
    speContainer: true|false
```

### Step 3: Create constraint files

Create `constraints/` directory with appropriate constraint manifests:
- `pss_baseline.yaml` for baseline tests
- `policy_1.yaml` for security policy tests

### Step 4: Create test-matrix.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: <your-constraint>
spec:
  suiteName: d8-<your-constraint>
  outputTestDirectory: rendered
  defaultObjectBase: admissionPod
  defaultInventory:
    - ref: ../../../_test-samples/ns.yaml
  bases:
    admissionPod:
      document:
        apiVersion: v1
        kind: Pod
        metadata:
          namespace: testns
        spec:
          containers:
            - image: nginx
              name: nginx
    securityPolicyException:              # Only if SPE is supported
      document:
        apiVersion: deckhouse.io/v1alpha1
        kind: SecurityPolicyException
        metadata:
          namespace: testns
  blocks:
    - name: <block-name>
      gatorBlock: <block-name>
      template: ../rendered/constraint-template.yaml
      constraint: constraints/<constraint-file>.yaml
      cases:
        - name: <case-name>
          violations: "yes"|"no"
          fields:
            - path: <field-path>
              scenario: <scenario>
          object:
            base: admissionPod
            merge:
              metadata:
                name: <case-name>
              spec: ...
```

### Step 5: Create test_profile.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <your-constraint>
spec:
  testDirectory: <your-constraint>
  suite:
    expectedTestBlockNames:
      - <block-name-from-matrix>
```

### Step 6: Generate, verify, test

```bash
# From the constraint directory:
constraint_testgen=../../tools/constraint_testgen

# Generate rendered artifacts
go run $constraint_testgen generate -bundle ./test-matrix.yaml

# Verify profile
go run $constraint_testgen verify

# Run gator tests
gator verify -v ./rendered

# Check coverage
go run $constraint_testgen coverage -tests-root ./ -format table
```

### Step 7: Iterate

Repeat generate → inspect rendered → coverage → gator until:
- No missing required scenarios
- All gator tests pass
- Test behavior matches policy intent

---

## 10. Step-by-step: adding tests to an existing constraint

### Step 1: Understand the current state

```bash
# Check current coverage
go run $constraint_testgen coverage -tests-root ./ -format table
```

Look for missing scenarios in the output.

### Step 2: Add cases to test-matrix.yaml

For each missing scenario, add a new case (or add `fields` annotations to existing cases):

```yaml
- name: <descriptive-case-name>
  violations: "yes"|"no"
  fields:
    - path: <field-path-from-test_fields>
      scenario: <missing-scenario>
  object:
    base: admissionPod
    merge:
      metadata:
        name: <descriptive-case-name>
      spec: ...
```

### Step 3: Update test_fields.yaml if needed

If the Rego was updated to check new fields, add them to `test_fields.yaml`.

### Step 4: Regenerate and verify

```bash
go run $constraint_testgen generate -bundle ./test-matrix.yaml
gator verify -v ./rendered
go run $constraint_testgen coverage -tests-root ./ -format table
```

---

## 11. Useful commands

All commands assume you are in the **module root** (`modules/015-admission-policy-engine`):

```bash
# Set tool path
constraint_testgen=./tools/constraint_testgen

# Generate rendered artifacts from matrix (single constraint)
go run $constraint_testgen generate \
  -bundle ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>/test-matrix.yaml

# Generate all constraints at once
go run $constraint_testgen generate -all \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints

# Verify constraint profiles
go run $constraint_testgen verify \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints

# Check coverage (table format)
go run $constraint_testgen coverage \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints -format table

# Check coverage (JSON format)
go run $constraint_testgen coverage \
  -tests-root ./charts/constraint-templates/tests/test_cases/constraints -format json

# Run gator verification for a single constraint
cd ./charts/constraint-templates/tests/test_cases/constraints/<group>/<constraint>
gator verify -v ./rendered

# Run all tests (OPA library + gator + coverage)
./charts/constraint-templates/tests/test_cases/run_all_tests.sh
```

### From within a constraint directory

```bash
# Set tool path (relative from constraint dir)
constraint_testgen=../../../../tools/constraint_testgen

# Generate
go run $constraint_testgen generate -bundle ./test-matrix.yaml

# Verify
go run $constraint_testgen verify

# Gator
gator verify -v ./rendered

# Coverage
go run $constraint_testgen coverage -tests-root ./ -format table
```

### Prerequisites

The following tools must be installed:
- `go` — Go compiler (for running constraint_testgen)
- `gator` — OPA Gatekeeper CLI (`go install github.com/open-policy-agent/gatekeeper/v3/cmd/gator@latest`)
- `opa` — Open Policy Agent CLI (for OPA library tests)
- `python3` — used by the test runner for coverage parsing

---

## 12. Known limitations

1. **Array merge semantics**: arrays in `merge` patches **replace** the entire array at that path. Use `containerMerges` / `initContainerMerges` for targeted container patches.

2. **Generated files**: never hand-edit files under `rendered/`. They are overwritten on every `generate` run.

### Testing external_data constraints

Constraints that use `external_data` (e.g. `verify-image-signature`, `vulnerable-images`) **can** be tested with gator using the inventory-based mock pattern. The approach:

1. **Rego template** includes an `isTest` parameter. When `isTest: true`, the template calls `external_data_from_inventory(provider, keys)` instead of the real `external_data` function. This helper reads mock responses from gator inventory objects.

2. **Constraint manifest** sets `parameters.isTest: true`:

   ```yaml
   spec:
     parameters:
       isTest: true
   ```

3. **test-matrix.yaml** declares mock provider responses at two levels:
   - `spec.externalData.providers` — default mock for all cases (copied into every generated case)
   - Per-case `externalData.providers` — override for specific cases (e.g. to simulate errors or vulnerabilities)

   ```yaml
   spec:
     externalData:
       providers:
         - name: trivy-provider
           errors: []
           system_error: ""
           responses:
             "nginx:latest":
               vulnerabilities: []
     blocks:
       - name: security-policy
         cases:
           - name: negative-image-reference
             violations: "yes"
             externalData:                    # Per-case override
               providers:
                 - name: trivy-provider
                   errors:
                     - "image contains high vulnerabilities"
                   system_error: ""
                   responses:
                     "vulnerable/nginx:latest":
                       vulnerabilities:
                         - severity: HIGH
                           id: CVE-TEST-0001
             object:
               base: admissionPod
               merge: ...
   ```

See `test_cases/constraints/security/vulnerable-images/test-matrix.yaml` and `test_cases/constraints/security/verify-image-signature/test-matrix.yaml` for complete working examples.

---

## 13. Troubleshooting

### Coverage says scenario is missing, but case exists

Most common cause: mismatch in `fields.path` or wrong `scenario` value. The path must be byte-for-byte equal to the `test_fields.yaml` path.

### Case tests behavior, but coverage is still low

The case probably has no `fields` annotation (or incomplete annotation). Coverage is annotation-driven.

### Generated files changed unexpectedly

This is normal after matrix edits. Treat `rendered/` as build output. Review for correctness, do not manually "fix" generated YAML.

### SPE cases fail unexpectedly

Re-check:
- SPE path correctness against CRD
- Whether the case really matches SPE selector/target
- Whether the case is mapped to SPE scenarios (`speMatch`, `speMismatch`, `speAbsent`, `speContainerSpecific`)
- Whether the Pod has the correct `security.deckhouse.io/security-policy-exception` label

### gator fails while matrix looks correct

Confirm generation was rerun after latest edits and `rendered/` is up to date. Then inspect the failing case fixture in `rendered/test_samples/` and compare with intended merge/base inputs.

---

## 14. Definition of Done

Mark a constraint as done only when all items are true:

- [ ] `test_profile.yaml` exists and defines required suite blocks in `spec.suite.expectedTestBlockNames`
- [ ] `test_fields.yaml` contains all decision-driving object fields from Rego
- [ ] `test_fields.yaml` contains all decision-driving SPE fields used by the constraint
- [ ] `level`, `defaultBehavior`, and `applicableTracks` are correct
- [ ] Every required field scenario has at least one mapped case in `test-matrix.yaml`
- [ ] `constraint_testgen generate` succeeds and `rendered/` is refreshed
- [ ] `constraint_testgen verify` passes (profile is valid, required test blocks present)
- [ ] Coverage shows no missing required scenarios
- [ ] gator verification passes for the constraint test set
- [ ] Case names and `fields` annotations are understandable for a newcomer reviewer
