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

## Running Tests

```bash
# Run a specific test
chainsaw test --test-dir ./low-node-utilization/

# Run all tests recursively from current directory
chainsaw test

# Run with verbose output (full test path in logs)
chainsaw test --test-dir ./low-node-utilization/ --full-name

# Dry run — validate YAML without executing
chainsaw test --test-dir ./low-node-utilization/ --no-cluster

# Skip cleanup — keep created resources for debugging
chainsaw test --test-dir ./low-node-utilization/ --skip-delete

# Stop on first failure
chainsaw test --test-dir ./low-node-utilization/ --fail-fast

# Run tests in parallel (default: unlimited)
chainsaw test --parallel 4

# Override timeouts
chainsaw test --test-dir ./low-node-utilization/ \
  --apply-timeout 60s \
  --assert-timeout 300s \
  --exec-timeout 300s

# Generate test report
chainsaw test --test-dir ./low-node-utilization/ \
  --report-format JSON \
  --report-name chainsaw-report \
  --report-path ./reports/

# Pause on failure (for interactive debugging)
chainsaw test --test-dir ./low-node-utilization/ --pause-on-failure
```

**Key concepts:**
- `try` — main operations; step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `finally` — runs always (cleanup/teardown)
- `$NAMESPACE` — auto-generated test namespace, available in scripts

## Available Tests

| Directory | Description |
|-----------|-------------|
| `low-node-utilization/` | Validates LowNodeUtilization plugin rebalances pods from overloaded nodes |
| `high-node-utilization/` | Validates HighNodeUtilization plugin consolidates pods to fewer nodes |
| `exclude-namespaces-from-processing/` | Validates Deckhouse patch preventing eviction of pods in `d8-*` and `kube-system` namespaces |
