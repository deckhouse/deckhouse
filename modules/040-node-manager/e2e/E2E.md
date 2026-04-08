# E2E Testing for Cluster Autoscaler with Kyverno Chainsaw

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
- `jq` installed on the machine running tests
- Sufficient RBAC permissions to create/delete NodeGroups, InstanceClasses, and Deployments
- Cluster Autoscaler deployed in `d8-cloud-instance-manager` namespace
- **DVP tests:** working `DVPInstanceClass` named `worker` must exist
- **Yandex tests:** working `YandexInstanceClass` named `worker-small` must exist

## Running Tests

```bash
# Run a specific test
chainsaw test --test-dir ./ca-priority-fallback-dvp/

# Run all tests recursively from current directory
chainsaw test

# Dry run — validate YAML without executing
chainsaw test --test-dir ./ca-scale-from-zero-dvp/ --no-cluster

# Skip cleanup — keep created resources for debugging
chainsaw test --test-dir ./ca-scale-from-zero-dvp/ --skip-delete

# Pause on failure (for interactive debugging)
chainsaw test --test-dir ./ca-priority-fallback-dvp/ --pause-on-failure

# Override timeouts (useful for slow environments)
chainsaw test --test-dir ./ca-priority-fallback-yandex/ --exec-timeout 7200s
```

**Key concepts:**
- `try` — main operations; step fails if any operation fails
- `catch` — runs only on failure (diagnostics collection)
- `finally` — runs always (cleanup/teardown)
- `$NAMESPACE` — auto-generated test namespace, available in scripts

## Available Tests

| Directory | Cloud Provider | Scenario | Estimated Duration |
|-----------|---------------|----------|-------------------|
| `ca-priority-fallback-dvp/` | DVP (CAPI) | Priority fallback on broken InstanceClass | ~25 min |
| `ca-priority-fallback-yandex/` | Yandex Cloud (MCM) | Priority fallback on broken InstanceClass | ~55 min |
| `ca-scale-from-zero-dvp/` | DVP (CAPI) | Scale from zero with priority selection | ~10 min |
| `ca-scale-from-zero-yandex/` | Yandex Cloud (MCM) | Scale from zero with priority selection | ~15 min |

## Test Scenarios

### Priority Fallback (ca-priority-fallback-*)

Tests the Priority Expander and backoff mechanism:
1. Creates a broken InstanceClass (cloned from a working one with an invalid field)
2. Creates two NodeGroups: priority 100 (broken IC) and priority 50 (working IC)
3. Creates a Deployment with 3 replicas, nodeSelector, tolerations, and podAntiAffinity
4. Verifies CA selects the priority 100 group first
5. Waits for CA to backoff from the broken group (~15 min for CAPI, ~45 min for MCM with 3 zones)
6. Verifies CA falls back to the priority 50 group
7. Verifies all pods are running on nodes from the priority 50 group

### Scale from Zero (ca-scale-from-zero-*)

Tests basic scale-from-zero with Priority Expander:
1. Creates two NodeGroups with working ICs: priority 100 and priority 50, both with minPerZone: 0
2. Creates a Deployment with 3 replicas, nodeSelector, tolerations, and podAntiAffinity
3. Verifies CA selects the priority 100 group
4. Verifies all pods are running on nodes from the priority 100 group
