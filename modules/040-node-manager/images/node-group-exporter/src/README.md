# Node Group Exporter

Prometheus exporter for monitoring Node Groups from Deckhouse. The exporter collects metrics about the number of nodes in each group and information about the nodes themselves.

## Features

- Subscribe to Node and NodeGroup resource changes
- Metrics for the number of nodes in each group
- Node information metrics per group
- HTTP server for Prometheus metrics export
- Graceful shutdown support

## Metrics

### node_group_count_nodes_total
Total number of nodes in a node group.

```
node_group_count_nodes_total{node_group="worker", node_type="Cloud"} 5
```

### node_group_count_ready_total
Number of ready nodes in a node group.

```
node_group_count_ready_total{node_group="worker", node_type="Cloud"} 4
```

### node_group_count_max_total
Maximum number of nodes allowed in a node group.

```
node_group_count_max_total{node_group="worker", node_type="Cloud"} 10
```

### node_group_node
Information about individual nodes in groups (1 if node exists and is Ready, 0 otherwise).

```
node_group_node{node_group="worker", node_type="Cloud", node="worker-1"} 1
```

## Usage

```bash
# Run with default settings
./node-group-exporter

# Run on custom port
./node-group-exporter -server.exporter-address=:8080

# Run with debug logging
./node-group-exporter -server.log-level=debug
```

## Development

### Requirements

- Go 1.21+

### Build

```bash
# Install dependencies
make tidy

# Run tests
make test

# Run locally
make run
```

## Monitoring

### Health check

The exporter provides `/health` and `/healthz` endpoints for health checks:

```bash
curl http://localhost:9000/health
```

### Metrics

Metrics are available at `/metrics`:

```bash
curl http://localhost:9000/metrics
```

## License

Apache-2.0
