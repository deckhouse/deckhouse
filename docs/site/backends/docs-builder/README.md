# Docs Builder

A Go-based HTTP service built on [Hugo](https://gohugo.io/) that manages the complete documentation lifecycle for Deckhouse Platform Certified Security Edition modules. The service handles documentation uploading, building, and serving with support for multiple publication channels and languages.

## Architecture Overview

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Client API    │───▶│   Docs Builder   │───▶│   Hugo Engine   │
│  (Upload/Build) │    │     Service      │    │   (Generation)  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │  Static Files    │
                       │   (Serve/Host)   │
                       └──────────────────┘
```

Docs Builder handles the complete documentation lifecycle:
- **Reception:** Accepting module documentation uploads as tar archives
- **Processing:** Extracting and organizing content by module, version, and channel
- **Building:** Generating static sites using Hugo with multilingual support (EN/RU)
- **Hosting:** Serving documentation for end users
- **High Availability:** Leader election for multi-instance deployments

## Operating Modes

### 1. Frontend Mode
**Purpose:** Public-facing documentation website
**Example:** [Deckhouse documentation site](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-overview.html)
**Features:**
- Full module documentation across all channels
- Public internet accessibility
- Complete feature documentation

### 2. Cluster Documentation Mode
**Purpose:** Internal cluster documentation
**Features:**
- Kubernetes cluster-internal access only
- Documentation limited to installed modules
- Runtime-specific configuration examples

## API Reference

### Documentation Management
| Method | Endpoint | Description | Parameters |
|--------|----------|-------------|------------|
| `GET`  | `/api/v1/doc` | Retrieve all modules with versions and channels | None |
| `POST` | `/api/v1/doc/{moduleName}/{version}` | Upload module documentation tar archive | `channels` (query): comma-separated channel list (default: stable) |
| `DELETE` | `/api/v1/doc/{moduleName}` | Remove module documentation | `channels` (query): comma-separated channel list (default: stable) |
| `POST` | `/api/v1/build` | Trigger complete site rebuild | None |

### Health & Status
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`  | `/readyz` | Readiness probe - returns 200 when service is ready to serve traffic |
| `GET`  | `/healthz` | Health probe - returns 200 when service is running |

### Request Examples
```bash
# Upload documentation
curl -X POST \
  'http://localhost:8081/api/v1/doc/mymodule/v1.2.3?channels=alpha,beta' \
  --data-binary @module-docs.tar

# Get all documentation info
curl http://localhost:8081/api/v1/doc

# Trigger rebuild
curl -X POST http://localhost:8081/api/v1/build

# Delete module documentation
curl -X DELETE 'http://localhost:8081/api/v1/doc/mymodule?channels=alpha,beta'
```

## Configuration

### Command-Line Flags
| Flag | Default | Description |
|------|---------|-------------|
| `--address` | `:8081` | HTTP server bind address and port |
| `--src` | `/app/hugo/` | Hugo source files directory |
| `--dst` | `/mount/` | Built site output directory |
| `--highAvailability` | `false` | Enable HA mode with Kubernetes leader election |

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Logging level: debug, info, warn, error, fatal |
| `POD_NAME` | | Pod name for HA mode (required in HA) |
| `POD_IP` | | Pod IP for HA mode (required in HA) |
| `POD_NAMESPACE` | | Pod namespace for HA mode (required in HA) |
| `CLUSTER_DOMAIN` | | Kubernetes cluster domain for HA mode |

## Directory Structure

```
docs-builder/
├── main.go                     # Application entry point
├── go.mod                      # Go module dependencies
├── internal/
│   ├── docs/                   # Core documentation service
│   │   ├── service.go          # Main service logic
│   │   ├── build.go            # Hugo build functionality
│   │   ├── upload.go           # Documentation upload handling
│   │   ├── delete.go           # Documentation removal
│   │   ├── info.go             # Documentation info retrieval
│   │   └── channel_mapping.go  # Channel version management
│   └── http/v1/
│       └── handler.go          # HTTP API handlers
└── pkg/
    ├── hugo/                   # Hugo integration
    │   ├── hugobuilder.go      # Hugo build wrapper
    │   ├── commandeer.go       # Hugo command interface
    │   └── server.go           # Hugo server mode
    └── k8s/
        └── manager.go          # Kubernetes lease management
```

## File Processing

### Upload Archive Structure
The service expects tar archives with the following structure:
```
module-archive.tar
├── docs/                       # Content files → content/modules/{module}/{channel}/
│   ├── README.md              # Module documentation
│   ├── FAQ.md                 # FAQ documentation
│   └── configuration.md       # Configuration docs
├── openapi/                   # API specs → data/modules/{module}/{channel}/
│   ├── config-values.yaml     # Configuration schema
│   └── doc-*-config-values.yaml # Generated config docs
└── crds/                      # CRDs → data/modules/{module}/{channel}/
    └── *.yaml                 # Custom Resource Definitions
```

### Language Support
- **English:** Default documentation files (`.md`)
- **Russian:** Files with `_RU.md` suffix (converted to `.ru.md`)

### Channel Management
- Supports multiple publication channels (stable, alpha, beta, etc.)
- Version mapping maintained in `data/channel-mapping.yaml`
- Automatic cleanup of broken modules during build

## High Availability Features

When `--highAvailability` is enabled:

1. **Leader Election:** Uses Kubernetes coordination leases
2. **Lease Management:**
   - Lease duration: 35 seconds
   - Renew period: 30 seconds
   - Garbage collection: 90 seconds
3. **Service Discovery:** Automatic pod address registration
4. **Graceful Shutdown:** Lease cleanup on termination

## Error Handling

### Build Error Recovery
- **Broken Module Detection:** Automatic parsing of Hugo build errors
- **Cleanup Strategy:** Removes corrupted modules and rebuilds
- **Channel Mapping Updates:** Maintains consistency after cleanup
- **Logging:** Comprehensive error tracking and module removal logging

### Validation
- **Path Traversal Protection:** Prevents malicious archive paths
- **File Type Filtering:** Processes only supported file types
- **Permission Management:** Secure file permission handling (user-only access)

## Dependencies

### Core Dependencies
- **Hugo:** v0.150.1 - Static site generator
- **Kubernetes Client:** v0.28.3 - K8s API integration
- **Deckhouse Logger:** Custom structured logging
- **Afero/Fsync:** File system operations

### Key Features from Dependencies
- **Hugo Extensions:** Goldmark, i18n, asset processing
- **File Operations:** Atomic file syncing, overlay filesystems
- **Kubernetes Integration:** Lease-based coordination, service discovery
