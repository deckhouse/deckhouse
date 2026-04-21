# E2E Testing with Kyverno Chainsaw

## What is Chainsaw

[Chainsaw](https://kyverno.github.io/chainsaw/) is a declarative e2e testing tool for Kubernetes. Tests are defined in YAML as a sequence of steps with `try`/`catch`/`finally` blocks. Chainsaw creates a temporary namespace per test, applies resources, runs assertions and scripts, and cleans up automatically.

## Installation

**Homebrew (macOS/Linux):**

```bash
brew tap kyverno/chainsaw https://github.com/kyverno/chainsaw
brew install kyverno/chainsaw/chainsaw
```

**Go install:**

```bash
go install github.com/kyverno/chainsaw@latest
```

**Verify:**

```bash
chainsaw version
```

## Prerequisites

- `kubectl` configured with access to a target Kubernetes cluster
- Sufficient RBAC permissions to create/delete namespaces and resources
- Descheduler module enabled with `deschedulingInterval: Frequent` (5m) in ModuleConfig for faster test cycles

## Running Tests

The recommended way to run tests is via [go-task](https://taskfile.dev/) using the provided `Taskfile.yml`:

```bash
# Run all tests
task run

# Run a specific test
task run:low-node-utilization
task run:high-node-utilization
task run:exclude-namespaces

# Run with verbose output
task run:verbose

# Dry run — validate YAML without executing (no cluster required)
task dry-run

# Pause on failure for debugging
task run:debug

# Generate a JSON report
task run:report
```

Alternatively, you can use `chainsaw` directly:

```bash
# Run all tests
chainsaw test --test-dir ./tests/

# Run a specific test
chainsaw test --test-dir ./tests/low-node-utilization/

# Skip cleanup — keep created resources for debugging
chainsaw test --test-dir ./tests/low-node-utilization/ --skip-delete

# Run tests in parallel (default: unlimited)
chainsaw test --test-dir ./tests/ --parallel 4

# Override timeouts
chainsaw test --test-dir ./tests/low-node-utilization/ \
  --apply-timeout 60s \
  --assert-timeout 300s \
  --exec-timeout 300s
```

**Key concepts:**
- `try` — main operations; step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `cleanup` — runs after step completes (resource deletion)
- `$NAMESPACE` — auto-generated test namespace, available in scripts

## Test Structure

```text
e2e/
  Taskfile.yml                  — Task runner for convenient test execution
  e2e.yaml                      — Chainsaw configuration
  tests/
    common/
      assert-descheduler-ready.yaml  — Shared assertion: descheduler deployment is ready
    low-node-utilization/
      chainsaw-test.yaml             — Test definition
      files/descheduler-cr.yaml      — Descheduler CR with LowNodeUtilization strategy
      low_node_utilization.md        — Test documentation
    high-node-utilization/
      chainsaw-test.yaml
      files/descheduler-cr.yaml      — Descheduler CR with HighNodeUtilization strategy
      high_node_utilization.md
    exclude-namespaces-from-processing/
      chainsaw-test.yaml
      files/descheduler-cr.yaml      — Descheduler CR with LowNodeUtilization strategy
      files/protected-namespace.yaml — d8-chainsaw-test namespace definition
      exclude_namespaces_from_processing.md
```

## Available Tests

| Task command | Test directory | Description |
|--------------|----------------|-------------|
| `task run:low-node-utilization` | `tests/low-node-utilization/` | Validates LowNodeUtilization plugin rebalances pods from overloaded nodes |
| `task run:high-node-utilization` | `tests/high-node-utilization/` | Validates HighNodeUtilization plugin consolidates pods to fewer nodes |
| `task run:exclude-namespaces` | `tests/exclude-namespaces-from-processing/` | Validates Deckhouse patch preventing eviction of pods in `d8-*` and `kube-system` namespaces |
