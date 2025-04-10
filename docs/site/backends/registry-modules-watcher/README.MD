# Registry Modules Watcher

## What It Does

Registry Modules Watcher automatically synchronizes module data between container registries and the documentation builder service, ensuring your documentation stays current with module changes.

## How It Works

### Synchronization Process

The service performs these steps at regular intervals:

1. Fetches current module listings from all configured registries
2. Compares registry data with its internal cache
3. Updates the documentation builder based on detected changes:
    - ✅ **New modules**: Sends creation requests for new modules
    - 🔄 **Updated modules**: Replaces outdated module versions
    - ❌ **Deleted modules**: Removes modules no longer in registry
    - ⏩ **Unchanged modules**: Skips modules with no changes

### Initial Startup Behavior

When starting with an empty cache:
- Service builds its initial registry data cache
- No updates are sent to docs-builder
- This prevents unnecessary rebuilds after service restarts

## Configuration Guide

### Command-Line Options

| Flag | Required | Default | Description |
|------|:--------:|---------|-------------|
| `--watch-registries` | ✅ | - | Comma-separated list of registries to monitor |
| `--scan-interval` |  | `15m` | Time between registry scans (e.g., `5m`, `1h`) |

### Environment Variables

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `REGISTRY_AUTHS` | ✅ | - | JSON object with registry credentials (Docker config format) |
| `POD_NAMESPACE` | ✅ | - | Kubernetes namespace of the running service |
| `LOG_LEVEL` |  | `info` | Logging level (`debug`, `info`, `warn`, `error`, `fatal`) |

## Deployment Notes

- Requires Kubernetes in-cluster configuration
- Uses service account for K8s API access and lease management
- Automatically manages sync state across restarts and multiple instances
