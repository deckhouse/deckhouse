# Registry Modules Watcher

A Kubernetes-native service that automatically synchronizes Deckhouse module documentation between container registries and documentation builder backends. It monitors container registries for module changes and keeps documentation services updated in real-time.

## Architecture Overview

The service consists of several core components:

- **Registry Scanner**: Monitors container registries for module changes using configurable scan intervals
- **Cache System**: Maintains state of known modules with checksum-based change detection
- **Backend Manager**: Coordinates updates to multiple documentation builder instances
- **Kubernetes Watcher**: Discovers and manages documentation builder backends via Kubernetes leases
- **HTTP/Metrics Server**: Provides health checks and Prometheus metrics on ports 8080 and 9090

## How It Works

### Module Synchronization Process

The service continuously monitors container registries for Deckhouse modules:

1. **Registry Scanning**: Periodically fetches module listings from all configured registries
2. **Change Detection**: Compares registry data with cached state using checksums
3. **Task Generation**: Creates documentation tasks for detected changes:
   - ✅ **New modules**: Creates documentation for newly discovered modules/versions
   - 🔄 **Updated modules**: Updates documentation when module content changes
   - ❌ **Deleted modules**: Removes documentation for modules no longer in registry
   - ⏩ **Unchanged modules**: Skips processing for modules with no changes
4. **Backend Distribution**: Sends tasks to all active documentation builder backends
5. **Cache Update**: Updates internal cache to reflect new registry state

### Release Channel Support

The service supports Deckhouse release channels (`alpha`, `beta`, `early-access`, `rock-solid`, `stable`) and:
- Tracks checksums per release channel to detect content changes
- Manages module versions across multiple release channels
- Ensures documentation builder receives proper channel information

### Backend Discovery

Uses Kubernetes lease objects with label `deckhouse.io/documentation-builder-sync` to:
- Automatically discover documentation builder instances
- Handle backend addition/removal without service restart
- Send full module state to newly discovered backends
- Maintain resilience across backend failures

### Startup Behavior

On initial startup with empty cache:
- Builds internal cache from registry data
- No updates sent to documentation builders
- Prevents unnecessary rebuilds after service restarts
- Subsequent scans detect real changes and trigger updates

## Configuration

### Command-Line Flags

| Flag | Required | Default | Description |
|------|:--------:|---------|-------------|
| `--watch-registries` | ✅ | - | Comma-separated list of container registries to monitor |
| `--scan-interval` |  | `15m` | Interval between registry scans (e.g., `5m`, `1h`, `30s`) |

### Environment Variables

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `REGISTRY_AUTHS` | ✅ | - | JSON object with registry credentials (Docker config format) |
| `POD_NAMESPACE` | ✅ | - | Kubernetes namespace for lease discovery and service operations |
| `LOG_LEVEL` |  | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`, `fatal`) |

### Example Usage

```bash
./registry-modules-watcher \
  --watch-registries="registry.deckhouse.io/deckhouse/modules,ghcr.io/deckhouse/modules" \
  --scan-interval=10m
```

## Monitoring & Observability

### HTTP Endpoints

- **Port 8080**: Health check and service status
- **Port 9090**: Prometheus metrics endpoint

### Key Metrics

| Metric | Description |
|--------|-------------|
| `registry_watcher_backends_total` | Number of active documentation builder backends |
| `registry_watcher_new_backends_total` | Count of newly discovered backends |
| `registry_scanner_cache_length` | Number of cached modules per registry |
| `registry_request_seconds` | Registry API response times |
| `sender_*_requests_*` | Documentation builder API request metrics |

## Deployment Requirements

### Kubernetes Configuration

- **In-cluster deployment**: Requires Kubernetes service account with appropriate RBAC
- **Lease access**: Needs permissions to read coordination.k8s.io/v1/Lease objects
- **Namespace scope**: Discovers backends within the same namespace

### RBAC Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: registry-modules-watcher
rules:
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch"]
```

### Resource Requirements

- **CPU**: Minimal - mostly I/O bound operations
- **Memory**: Scales with number of modules and cached documentation
- **Network**: Requires outbound access to container registries
- **Storage**: Ephemeral - all state maintained in memory cache

## API Integration

### Documentation Builder API

The service integrates with documentation builders via HTTP API:

- `POST /api/v1/doc/{module}/{version}?channels={channels}` - Upload module documentation
- `DELETE /api/v1/doc/{module}?channels={channels}` - Remove module documentation  
- `POST /api/v1/build` - Trigger documentation rebuild

### Error Handling

- **Exponential backoff**: Automatic retry for failed API calls
- **Circuit breaker**: Prevents cascade failures during backend issues
- **Graceful degradation**: Continues monitoring even if backends are unavailable
