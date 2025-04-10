# Docs Builder

A service built on [Hugo](https://gohugo.io/) that manages documentation for Deckhouse modules.

## Overview

Docs Builder handles the complete documentation lifecycle:
- Receiving module documentation
- Building static content
- Hosting documentation for users

## Operating Modes

### 1. Frontend Mode 
**Purpose:** Serves as a public-facing documentation website  
**Example:** [Deckhouse documentation site](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-overview.html)

### 2. Cluster Documentation Mode
**Purpose:** Internal documentation for cluster users  
**Features:**
- Accessible within the cluster only
- Displays documentation only for installed modules

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`  | `/api/v1/doc` | Get all modules, versions, and publication channels |
| `POST` | `/api/v1/doc/{moduleName}/{version}?channels=alpha,beta` | Upload documentation for module version |
| `DELETE` | `/api/v1/doc/{moduleName}?channels=alpha,beta` | Remove module documentation |
| `POST` | `/api/v1/build` | Trigger full documentation rebuild |

> **Note:** Multiple channels must be comma-separated

## Configuration

### Command-Line Options
| Option | Default | Required | Description |
|--------|---------|----------|-------------|
| `--address` | `:8081` |  | HTTP server address:port |
| `--src` | `/app/hugo/` |  | Source files location |
| `--dst` | `/mount/` |  | Built site output directory |
| `--highAvailability` | `false` |  | Enable high availability mode |

### Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `LOG_LEVEL` | info |  | Set logging level (debug, info, warn, error, fatal) |

## Functionality

- **HTTP Server:** Listens for API requests
- **Documentation Management:** Handles file operations in source directory
- **Site Generation:** Builds and outputs static files to destination
- **High Availability:** Uses Kubernetes leases for leader election when enabled
