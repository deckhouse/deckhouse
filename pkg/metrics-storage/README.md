# metrics-storage

A flexible metrics management package for Golang applications that provides a convenient and type-safe way to work with Prometheus metrics.

## Overview

The `metrics-storage` package is a wrapper around Prometheus client libraries that simplifies the process of registering, updating, and collecting metrics. It provides:

- Centralized metrics management
- Type-safe operations on metrics
- Grouped metrics that can be expired together
- Batch operations for efficient metrics updates
- Support for all common Prometheus metric types (Gauge, Counter, Histogram)

## Installation

```bash
go get github.com/deckhouse/deckhouse/pkg/metrics-storage
```

## Basic Usage

```go
package main

import (
    "context"
    "log/slog"
	
    "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func main() {
	// Create a new metrics storage with prefix "app"
	logger := log.Default()
	storage := metricsstorage.NewMetricStorage("app", WithNewRegistry(), WithLogger(logger))
	
	// Set a gauge value
	storage.GaugeSet("server_uptime_seconds", 3600.0, map[string]string{
		"instance": "web-1",
		"region": "us-west",
	})
	
	// Increment a counter
	storage.CounterAdd("http_requests_total", 1.0, map[string]string{
		"method": "GET",
		"path": "/api/v1/users",
		"status": "200",
	})
	
	// Record a histogram observation
	storage.HistogramObserve("request_duration_seconds", 0.42, map[string]string{
		"endpoint": "/api/v1/users",
	}, []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0})
}
```

## Creating a MetricStorage

```go
// Create with default registry
storage := metricsstorage.NewMetricStorage(ctx, "app")

// Create with new isolated registry
storage := metricsstorage.NewMetricStorage(ctx, "app", WithNewRegistry())
```

## Working with Metrics

### Gauges

Gauges represent a single numerical value that can go up and down.

```go
// Register a gauge with labels
gaugeVec := storage.RegisterGauge(
    "memory_usage_bytes",
    map[string]string{
        "instance": "",
        "component": "",
})

// Set gauge value
storage.GaugeSet("memory_usage_bytes", 1024*1024*100, map[string]string{
    "instance": "server-1",
    "component": "api",
})

// Add to gauge value (can be negative)
storage.GaugeAdd("memory_usage_bytes", 1024*1024, map[string]string{
    "instance": "server-1",
    "component": "api",
})
```

### Counters

Counters represent cumulative values that only increase.

```go
// Register a counter with labels and help text
counterVec := storage.RegisterCounter(
    "http_requests_total", 
    map[string]string{
        "method": "",
        "path": "",
        "status": "",
})

// Increment counter
storage.CounterAdd("http_requests_total", 1.0, map[string]string{
    "method": "POST",
    "path": "/api/users",
    "status": "201",
})
```

### Histograms

Histograms track the distribution of values.

```go
// Register a histogram with custom buckets
histogramVec := storage.RegisterHistogram(
    "request_duration_seconds",
    map[string]string{
        "method": "",
        "endpoint": "",
    },
    []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
)

// Record an observation
storage.HistogramObserve(
    "request_duration_seconds",
    0.42,
    map[string]string{
        "method": "GET",
        "endpoint": "/api/v1/users",
    },
    []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
)
```

## Batch Operations

The package supports batch operations for efficient updates:

```go
import (
    "k8s.io/utils/ptr"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

func updateMetrics(storage *MetricStorage) {
	operations := []operation.MetricOperation{
		{
			Name:   "http_requests_total",
			Action: operation.ActionAdd,
			Value:  ptr.To(1.0),
			Labels: map[string]string{
				"method": "GET",
				"path":   "/api/users",
			},
		},
		{
			Name:   "memory_usage_bytes",
			Action: operation.ActionSet,
			Value:  operation.ptr.To(1024 * 1024 * 150),
			Labels: map[string]string{
				"instance": "server-1",
			},
		},
	}

	// Apply all operations with common labels
	commonLabels := map[string]string{
		"environment": "production",
	}

	storage.ApplyBatchOperations(operations, commonLabels)
}

```

## Grouped Metrics

Grouped metrics can be expired together, which is useful for reporting metrics that should be cleared and replaced as a set:

```go
// Get the grouped interface
groupedMetrics := storage.Grouped()

// Add counter to a group
groupedMetrics.CounterAdd("user_stats", "logins_total", 1.0, map[string]string{
    "user_id": "12345",
})

// Set gauge in a group
groupedMetrics.GaugeSet("user_stats", "session_duration_seconds", 3600.0, map[string]string{
    "user_id": "12345",
})

// Later, expire all metrics in the group
groupedMetrics.ExpireGroupMetrics("user_stats")
```

## Using With Operations

You can also use the operations API directly:

```go
import (
    "k8s.io/utils/ptr"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func updateWithOperations(storage *metricsstorage.MetricStorage) {
    // Create an operation
    op := operation.MetricOperation{
        Name: "api_requests_total",
        Action: operation.ActionAdd,
        Value: ptr.To(1.0),
        Labels: map[string]string{
            "endpoint": "/users",
            "status": "200",
        },
    }
    
    // Apply the operation
    storage.ApplyOperation(op, map[string]string{
        "service": "user-api",
    })
    
    // Create a grouped operation
    groupedOp := operation.MetricOperation{
        Name: "active_sessions",
        Group: "session_metrics",
        Action: operation.ActionSet,
        Value: ptr.To(42.0),
        Labels: map[string]string{
            "region": "us-west",
        },
    }
    
    // Apply grouped operations in batch
    storage.ApplyBatchOperations([]operation.MetricOperation{groupedOp}, nil)
}
```

## Exposing Metrics

You can expose the metrics via HTTP for Prometheus to scrape:

```go
import (
    "net/http"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func main() {
    // Create storage with a new registry
    storage := metricsstorage.NewMetricStorage(ctx, "app", WithNewRegistry())
    
    // Configure your metrics...
    
    // Get HTTP handler for metrics endpoint
    handler := storage.Handler()
    
    // Create HTTP server
    http.Handle("/metrics", handler)
    http.ListenAndServe(":8080", nil)
}
```

## Advanced Features

### Using Prefix Templates

You can use template variables in metric names:

```go
// {PREFIX} will be replaced with the storage prefix
storage := metricsstorage.NewMetricStorage(ctx, "myapp")
storage.GaugeSet("{PREFIX}_component_status", 1.0, labels)  // becomes "myapp_component_status"
```

### Getting the Collector

If you need to register the metrics in an external registry:

```go
// Get the collector that can be registered elsewhere
collector := storage.Collector()
externalRegistry.MustRegister(collector)
```

## License

Apache 2.0 License