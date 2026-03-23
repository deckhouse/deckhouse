# AI Agent Guide: Constraint Template Tests

You are working with the Gatekeeper ConstraintTemplate testing framework for Deckhouse. This file tells you how to write, modify, and verify constraint tests.

## Essential references

Before writing any test, read these files:

| Resource | Path | Purpose |
|----------|------|---------|
| Testing guide (EN) | [`docs/TESTING_GUIDE.md`](docs/TESTING_GUIDE.md) | Comprehensive human documentation |
| Testing guide (RU) | [`docs/TESTING_GUIDE.ru.md`](docs/TESTING_GUIDE.ru.md) | Полное руководство (RU) |
| test_fields schema | [`openapi/constraint-test-fields.schema.yaml`](openapi/constraint-test-fields.schema.yaml) | Validation rules for `test_fields.yaml` |
| test-matrix schema | [`openapi/constraint-test-matrix.schema.yaml`](openapi/constraint-test-matrix.schema.yaml) | Validation rules for `test-matrix.yaml` |
| test_profile schema | [`openapi/constraint-test-profile.schema.yaml`](openapi/constraint-test-profile.schema.yaml) | Validation rules for `test_profile.yaml` |
| SPE CRD | `../../crds/security-policy-exception.yaml` | SecurityPolicyException field paths |

## Directory layout

```
tests/
├── test_cases/constraints/
│   ├── security/<constraint>/     # Security policy tests
│   │   ├── test_fields.yaml       # Field/scenario model (hand-written)
│   │   ├── test-matrix.yaml       # Test cases (hand-written)
│   │   ├── test_profile.yaml      # Suite/quality contract (hand-written)
│   │   ├── constraints/           # Constraint manifests (hand-written)
│   │   └── rendered/              # Generated artifacts (DO NOT EDIT)
│   └── operation/<constraint>/    # Operational policy tests (same structure)
├── tools/constraint_testgen/      # Code generator
└── openapi/                       # JSON Schema files
```

## Workflow for writing constraint tests

### Step 1: Understand the policy

1. Read the Rego template in `charts/constraint-templates/templates/<group>/<constraint>.yaml`.
2. Identify every object field the Rego reads (`input.review.object.*`).
3. Identify every SPE field the Rego reads (if applicable).
4. Check the SPE CRD for exact field paths — the shape in tests must match what the template expects.

### Step 2: Create test_fields.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestFields
metadata:
  name: <constraint-name>           # Must match directory name
spec:
  objectKind: Pod
  objectFields:
    - path: <exact.field.path>       # e.g. spec.hostNetwork
      level: pod|container|initContainer
      description: "<what this field does>"
      # requiredScenarios: omit to use defaults for the level
  speFields:                         # Only if SPE is supported
    - path: <spe.field.path>         # e.g. spec.network.hostNetwork.allowedValue
      level: pod|container
      description: "<what this SPE field does>"
  applicableTracks:
    functional: true
    spePod: true|false
    speContainer: true|false
```

**Default scenarios by level:**
- `pod` object field → `[positive, negative, absent]`
- `container` object field → `[positive, negative, absent, multiContainer, initContainer, ephemeralContainer]`
- `initContainer` object field → `[positive, negative, absent]`
- `pod` SPE field → `[speMatch, speMismatch, speAbsent]`
- `container` SPE field → `[speMatch, speMismatch, speAbsent, speContainerSpecific]`

### Step 3: Create test-matrix.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: <constraint-name>
spec:
  suiteName: d8-<constraint-name>
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
    securityPolicyException:          # Only if SPE is supported
      document:
        apiVersion: deckhouse.io/v1alpha1
        kind: SecurityPolicyException
        metadata:
          namespace: testns
  namedExceptions: {}
  blocks:
    - name: <block-name>
      gatorBlock: <block-name>
      template: ../rendered/constraint-template.yaml
      constraint: constraints/<file>.yaml
      cases:
        - name: <case-name>
          violations: "yes"|"no"
          fields:
            - path: <field-path>      # Must exactly match test_fields.yaml
              scenario: <scenario>    # Must be a valid scenario name
          object:
            base: admissionPod
            merge:
              metadata:
                name: <case-name>
              spec: ...
```

### Step 4: Create test_profile.yaml

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestProfile
metadata:
  name: <constraint-name>
spec:
  testDirectory: <constraint-name>
  suite:
    expectedTestBlockNames:
      - <block-name-1>               # Must match gatorBlock/name from matrix
      - <block-name-2>
```

### Step 5: Generate and verify

```bash
constraint_testgen=../../../../tools/constraint_testgen

# Generate
go run $constraint_testgen generate -bundle ./test-matrix.yaml

# Gator verify
gator verify -v ./rendered

# Profile verify
go run $constraint_testgen verify

# Coverage
go run $constraint_testgen coverage -tests-root ./ -format table
```

## Critical rules

1. **Path matching**: every `fields[].path` in test-matrix.yaml must be **byte-for-byte identical** to the corresponding `path` in test_fields.yaml.
2. **Array merge semantics**: arrays in `merge` patches **replace** the entire array. Use `containerMerges`/`initContainerMerges` for targeted container patches.
3. **Never edit `rendered/`**: all files under `rendered/` are generated. Re-run `generate` after any matrix change.
4. **SPE label**: SPE cases must set `metadata.labels.security.deckhouse.io/security-policy-exception: <exception-name>` on the Pod, where `<exception-name>` matches the SPE `metadata.name`.
5. **Constraint naming**: use `pss_baseline.yaml`, `pss_restricted.yaml`, `policy_<n>.yaml` (numbering from 1).
6. **external_data constraints** (e.g. `verify-image-signature`, `vulnerable-images`) are tested via the inventory-based mock pattern: set `parameters.isTest: true` in the constraint manifest, declare `spec.externalData.providers` in the matrix for default mock responses, and use per-case `externalData.providers` overrides to simulate errors/vulnerabilities. The Rego template's `external_data_from_inventory` helper reads mock data from gator inventory instead of calling real `external_data`.

## SPE case pattern

```yaml
- name: allowed-by-exception-<description>
  violations: "no"
  fields:
    - path: spec.<spe-section>.<spe-field>
      scenario: speMatch
  inventory:
    - base: securityPolicyException
      merge:
        metadata:
          name: <exception-name>
        spec:
          <spe-section>:
            <spe-fields>: <values>
  object:
    base: admissionPod
    merge:
      metadata:
        labels:
          security.deckhouse.io/security-policy-exception: <exception-name>
        name: allowed-by-exception-<description>
      spec:
        <violating-fields>: <values>
```

## Example: complete security constraint (allow-host-network)

### test_fields.yaml

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

### test_profile.yaml

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

### test-matrix.yaml (abbreviated — functional block)

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ConstraintTestMatrix
metadata:
  name: allow-host-network
spec:
  suiteName: d8-host-network-ports
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
    securityPolicyException:
      document:
        apiVersion: deckhouse.io/v1alpha1
        kind: SecurityPolicyException
        metadata:
          namespace: testns
  blocks:
    - name: pss-baseline-functional
      gatorBlock: pss-baseline-functional
      template: ../rendered/constraint-template.yaml
      constraint: constraints/pss_baseline.yaml
      cases:
        - name: allowed-no-hostnetwork
          violations: "no"
          fields:
            - path: spec.hostNetwork
              scenario: absent
          object:
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
        - name: disallowed-hostnetwork-true
          violations: "no"
          fields:
            - path: spec.hostNetwork
              scenario: negative
          object:
            base: admissionPod
            merge:
              metadata:
                name: disallowed-hostnetwork-true
              spec:
                hostNetwork: true
                containers:
                  - image: nginx
                    name: nginx
                    ports:
                      - containerPort: 80
```

## Example: simple operation constraint (allowed-repos)

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

## Checklist before committing

- [ ] `test_fields.yaml` lists all object fields the Rego reads
- [ ] `test_fields.yaml` lists all SPE fields (if applicable)
- [ ] `test-matrix.yaml` has `fields` annotations on every case
- [ ] Every required scenario from `test_fields.yaml` is covered by at least one case
- [ ] `test_profile.yaml` lists all block names from the matrix
- [ ] `go run $constraint_testgen generate -bundle ./test-matrix.yaml` succeeds
- [ ] `gator verify -v ./rendered` passes
- [ ] `go run $constraint_testgen verify` passes
- [ ] `go run $constraint_testgen coverage -tests-root ./ -format table` shows 100%
- [ ] `rendered/` artifacts are committed alongside source files
