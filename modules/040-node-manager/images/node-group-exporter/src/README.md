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

### d8_node_group_ready
Number of ready nodes in a node group (compatible with hook/node_group_metrics.go).

```
d8_node_group_ready{node_group_name="worker"} 4
```

### d8_node_group_nodes
Number of Kubernetes nodes (in any state) in the group.

```
d8_node_group_nodes{node_group_name="worker"} 5
```

### d8_node_group_instances
Number of instances (in any state) in the group.

```
d8_node_group_instances{node_group_name="worker"} 5
```

### d8_node_group_desired
Number of desired machines in the group.

```
d8_node_group_desired{node_group_name="worker"} 5
```

### d8_node_group_min
Minimal amount of instances in the group.

```
d8_node_group_min{node_group_name="worker"} 1
```

### d8_node_group_max
Maximum amount of instances in the group.

```
d8_node_group_max{node_group_name="worker"} 10
```

### d8_node_group_up_to_date
Number of up-to-date nodes in the group.

```
d8_node_group_up_to_date{node_group_name="worker"} 4
```

### d8_node_group_standby
Number of overprovisioned instances in the group.

```
d8_node_group_standby{node_group_name="worker"} 1
```

### d8_node_group_has_errors
Whether the node group has errors (1 if error condition is True, 0 otherwise).

```
d8_node_group_has_errors{node_group_name="worker"} 0
```

## Usage

```bash
# Run with default settings
./node-group-exporter

# Run on custom port
./node-group-exporter -server.exporter-address=:8080

# Run with debug logging
./node-group-exporter -server.debug=true
```
