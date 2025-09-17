# metrics-storage

A comprehensive metrics management package for Golang applications that provides a convenient and type-safe way to work with Prometheus metrics in Deckhouse applications.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Creating MetricStorage](#creating-metricstorage)
- [Metric Operations](#metric-operations)
- [Metric Registration](#metric-registration)
- [Grouped Metrics](#grouped-metrics)
- [Batch Operations with Operations API](#batch-operations-with-operations-api)
- [Prefix Templates](#prefix-templates)
- [Prometheus Integration](#prometheus-integration)
- [Advanced Usage Patterns](#advanced-usage-patterns)
- [Package Structure](#package-structure)
- [API Reference](#api-reference)
- [Best Practices](#best-practices)
- [Common Patterns and Use Cases](#common-patterns-and-use-cases)
- [Migration Guide](#migration-guide)
- [Troubleshooting](#troubleshooting)
- [License](#license)
- [Contributing](#contributing)
- [Related Projects](#related-projects)

## Overview

The `metrics-storage` package is a sophisticated wrapper around Prometheus client libraries that simplifies metrics management through:

- **Centralized Metrics Management**: Single entry point for all metric operations with consistent naming and labeling
- **Grouped Metrics Support**: Metrics can be organized into logical groups that can be expired together for coordinated lifecycle management
- **Batch Operations**: Efficient bulk metric updates with the operations API to reduce overhead
- **Type-Safe Operations**: Strongly typed metric collectors with automatic label management and validation
- **Flexible Registration**: Support for custom registries, loggers, and metric options with comprehensive configuration
- **Prometheus Integration**: Full compatibility with Prometheus ecosystem including native handlers and collectors
- **Thread-Safe Design**: All operations are safe for concurrent use across multiple goroutines

## Architecture

The package consists of several key components:

- **MetricStorage**: Main interface for metric operations and registration with support for prefix templates
- **GroupedVault**: Internal storage that manages grouped metrics and collectors with automatic expiration
- **Collectors**: Type-safe metric collectors (Counter, Gauge, Histogram) with consistent value handling
- **Operations**: Batch operation system for efficient metric updates with validation and error handling
- **Options**: Configuration system for storage and metric registration with extensible option patterns
- **Labels**: Utilities for label manipulation, merging, and validation to ensure consistent metric labeling

## Installation

```go
import "github.com/deckhouse/deckhouse/pkg/metrics-storage"
```

## Quick Start

```go
package main

import (
    "github.com/deckhouse/deckhouse/pkg/log"
    metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func main() {
    // Create a new metrics storage with prefix "app"
    logger := log.NewLogger()
    storage := metricsstorage.NewMetricStorage("app", 
        metricsstorage.WithNewRegistry(), 
        metricsstorage.WithLogger(logger),
    )
    
    // Set a gauge value
    storage.GaugeSet("server_uptime_seconds", 3600.0, map[string]string{
        "instance": "web-1",
        "region":   "us-west",
    })
    
    // Increment a counter  
    storage.CounterAdd("http_requests_total", 1.0, map[string]string{
        "method": "GET",
        "path":   "/api/v1/users", 
        "status": "200",
    })
    
    // Record a histogram observation
    storage.HistogramObserve("request_duration_seconds", 0.42, map[string]string{
        "endpoint": "/api/v1/users",
    }, []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0})
}
```

## Creating MetricStorage

### Basic Creation

```go
// Create with default Prometheus registry
storage := metricsstorage.NewMetricStorage("myapp")

// Create with new isolated registry  
storage := metricsstorage.NewMetricStorage("myapp", metricsstorage.WithNewRegistry())

// Create with custom registry
registry := prometheus.NewRegistry()
storage := metricsstorage.NewMetricStorage("myapp", metricsstorage.WithRegistry(registry))

// Create with custom logger
logger := log.NewLogger().Named("metrics")
storage := metricsstorage.NewMetricStorage("myapp", metricsstorage.WithLogger(logger))
```

### Available Options

The storage supports several configuration options:

```go
import "github.com/deckhouse/deckhouse/pkg/metrics-storage"

// WithNewRegistry creates a new isolated Prometheus registry
storage := metricsstorage.NewMetricStorage("app", metricsstorage.WithNewRegistry())

// WithRegistry uses an existing registry  
registry := prometheus.NewRegistry()
storage := metricsstorage.NewMetricStorage("app", metricsstorage.WithRegistry(registry))

// WithLogger sets a custom logger
logger := log.NewLogger().Named("metrics")
storage := metricsstorage.NewMetricStorage("app", metricsstorage.WithLogger(logger))

// Multiple options can be combined
storage := metricsstorage.NewMetricStorage("app",
    metricsstorage.WithNewRegistry(),
    metricsstorage.WithLogger(logger),
)
```

## Metric Operations

### Counters

Counters represent cumulative values that only increase:

```go
// Simple counter increment
storage.CounterAdd("http_requests_total", 1.0, map[string]string{
    "method": "GET",
    "path":   "/api/users",
    "status": "200",
})

// Get a counter collector for direct operations
counter := storage.Counter("requests_processed", map[string]string{
    "service": "user-api",
})
// Use the counter collector directly
counter.Add("processing_group", 5.0, map[string]string{
    "service": "user-api",
    "status":  "success",
})
```

### Gauges

Gauges represent single numerical values that can go up and down:

```go
// Set gauge value
storage.GaugeSet("memory_usage_bytes", 1024*1024*100, map[string]string{
    "instance":  "server-1", 
    "component": "api",
})

// Add to gauge (can be negative)
storage.GaugeAdd("memory_usage_bytes", 1024*1024, map[string]string{
    "instance":  "server-1",
    "component": "api", 
})

// Get a gauge collector for direct operations
gauge := storage.Gauge("active_connections", map[string]string{
    "service": "database",
})
// Use the gauge collector directly
gauge.Set("connection_group", 42.0, map[string]string{
    "service": "database",
    "pool":    "main",
})
```

### Histograms

Histograms track the distribution of values:

```go
// Record histogram observation
buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
storage.HistogramObserve("request_duration_seconds", 0.42, map[string]string{
    "method":   "GET",
    "endpoint": "/api/v1/users",
}, buckets)

// Get a histogram collector for direct operations  
histogram := storage.Histogram("processing_time", map[string]string{
    "service": "worker",
}, buckets)
// Use the histogram collector directly
histogram.Observe("task_group", 1.23, map[string]string{
    "service": "worker",
    "task":    "data_processing",
})
```

## Metric Registration

For advanced use cases, you can explicitly register metrics:

```go
import "github.com/deckhouse/deckhouse/pkg/metrics-storage/options"

// Register a counter with custom options
counter, err := storage.RegisterCounter("api_calls_total", 
    []string{"method", "endpoint"}, 
    options.WithHelp("Total number of API calls"),
    options.WithConstantLabels(map[string]string{
        "service": "user-api",
    }),
)

// Register a gauge with custom options
gauge, err := storage.RegisterGauge("active_sessions",
    []string{"region", "datacenter"},
    options.WithHelp("Number of active user sessions"),
)

// Register a histogram with custom buckets and options
histogram, err := storage.RegisterHistogram("response_size_bytes",
    []string{"endpoint", "content_type"},
    []float64{100, 1000, 10000, 100000, 1000000},
    options.WithHelp("HTTP response size distribution"),
)
```

## Grouped Metrics

Grouped metrics allow you to organize related metrics that should be managed together:

```go
// Get the grouped interface
grouped := storage.Grouped()

// Add metrics to a group
grouped.CounterAdd("user_events", "login_total", 1.0, map[string]string{
    "user_id": "12345",
    "source":  "web",
})

grouped.GaugeSet("user_events", "session_duration_seconds", 3600.0, map[string]string{
    "user_id": "12345",
})

grouped.HistogramObserve("user_events", "request_latency", 0.15, map[string]string{
    "user_id": "12345",
    "action":  "profile_update", 
}, []float64{0.01, 0.1, 1.0, 10.0})

// Later, expire all metrics in the group
grouped.ExpireGroupMetrics("user_events")

// Or expire a specific metric in a group
grouped.ExpireGroupMetricByName("user_events", "login_total")
```

## Batch Operations with Operations API

The package supports efficient batch operations using the operations API:

```go
import (
    "k8s.io/utils/ptr"
    "github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
)

// Create individual operations
operations := []operation.MetricOperation{
    {
        Name:   "http_requests_total",
        Action: operation.ActionCounterAdd,
        Value:  ptr.To(1.0),
        Labels: map[string]string{
            "method": "GET",
            "path":   "/api/users",
        },
    },
    {
        Name:   "memory_usage_bytes", 
        Action: operation.ActionGaugeSet,
        Value:  ptr.To(float64(1024 * 1024 * 150)),
        Labels: map[string]string{
            "instance": "server-1",
        },
    },
    {
        Name:   "request_duration_seconds",
        Action: operation.ActionHistogramObserve,
        Value:  ptr.To(0.42),
        Labels: map[string]string{
            "endpoint": "/api/users",
        },
        Buckets: []float64{0.01, 0.1, 1.0, 10.0},
    },
}

// Apply operations with optional common labels
commonLabels := map[string]string{
    "environment": "production",
    "service":     "user-api",
}

storage.ApplyBatchOperations(operations, commonLabels)
```

### Grouped Operations

Operations can include group information for coordinated metric lifecycle management:

```go
// Create grouped operations
groupedOps := []operation.MetricOperation{
    {
        Name:   "active_users",
        Group:  "user_stats",  // All metrics in this group will be expired together
        Action: operation.ActionGaugeSet,
        Value:  ptr.To(150.0),
        Labels: map[string]string{
            "region": "us-west",
        },
    },
    {
        Name:   "session_count",
        Group:  "user_stats",  // Same group
        Action: operation.ActionGaugeSet, 
        Value:  ptr.To(89.0),
        Labels: map[string]string{
            "region": "us-west",
        },
    },
}

// Apply grouped operations - existing group metrics are expired first
storage.ApplyBatchOperations(groupedOps, nil)

// Individual operation with group
singleOp := operation.MetricOperation{
    Name:   "login_attempts",
    Group:  "security_metrics",
    Action: operation.ActionCounterAdd,
    Value:  ptr.To(1.0),
    Labels: map[string]string{
        "source": "web",
        "result": "success",
    },
}

storage.ApplyOperation(singleOp, map[string]string{
    "datacenter": "east-1",
})
```

### Available Actions

The operations API supports these actions:

```go
// Counter operations
operation.ActionCounterAdd   // Increment counter value

// Gauge operations  
operation.ActionGaugeSet     // Set gauge to specific value
operation.ActionGaugeAdd     // Add to current gauge value (can be negative)

// Histogram operations
operation.ActionHistogramObserve  // Record histogram observation

// Group management operations
operation.ActionExpireMetrics     // Expire all metrics in a group
```

## Prefix Templates

Metric names can use template variables for dynamic prefix resolution:

```go
// Create storage with prefix "myapp"
storage := metricsstorage.NewMetricStorage("myapp")

// Use {PREFIX} template in metric names
storage.GaugeSet("{PREFIX}_component_status", 1.0, map[string]string{
    "component": "database",
})
// Results in metric name: "myapp_component_status"

storage.CounterAdd("{PREFIX}_errors_total", 1.0, map[string]string{
    "type": "connection",
})
// Results in metric name: "myapp_errors_total"
```

## Prometheus Integration

### Exposing Metrics via HTTP

#### Using Built-in Handler

The simplest way to expose metrics is using the built-in `Handler()` method:

```go
import (
    "net/http"
    metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func main() {
    // Create storage
    storage := metricsstorage.NewMetricStorage("app")
    
    // Configure your metrics...
    storage.CounterAdd("requests_total", 1.0, map[string]string{
        "method": "GET",
        "path": "/api",
    })
    
    // Use the built-in handler - automatically handles registry configuration
    http.Handle("/metrics", storage.Handler())
    
    log.Println("Metrics server starting on :8080")
    http.ListenAndServe(":8080", nil)
}
```

#### Using Custom Registry

For more control, you can use a custom registry:

```go
import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    // Create storage with custom registry
    storage := metricsstorage.NewMetricStorage("app", metricsstorage.WithNewRegistry())
    
    // Configure your metrics...
    
    // Get the collector for the registry
    collector := storage.Collector()
    
    // Create HTTP handler for metrics endpoint
    registry := prometheus.NewRegistry()
    registry.MustRegister(collector)
    handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
    
    // Serve metrics
    http.Handle("/metrics", handler)
    http.ListenAndServe(":8080", nil)
}
```

### Using with External Registry

```go
// Get the registerer for external registration
registerer := storage.Registerer()

// Register additional metrics
customCounter := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "custom_metric_total", 
        Help: "A custom metric",
    },
    []string{"label1"},
)
registerer.MustRegister(customCounter)

// Get the collector for external registration
collector := storage.Collector()
externalRegistry.MustRegister(collector)
```

### Getting Prometheus Gatherer

```go
// For metrics collection and exposition
metrics, err := storage.Gather()
if err != nil {
    log.Printf("Failed to gather metrics: %v", err)
}
```

### Advanced Handler Configuration

For production use cases, you might want more control over the HTTP handler:

```go
import (
    "context"
    "net/http"
    "time"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func setupMetricsServer(storage *metricsstorage.MetricStorage) *http.Server {
    // Create a custom handler with timeout and other options
    handler := storage.Handler()
    
    // Wrap with middleware for logging, authentication, etc.
    wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        defer func() {
            duration := time.Since(start)
            log.Printf("Metrics request completed in %v", duration)
        }()
        
        // Add CORS headers if needed
        w.Header().Set("Access-Control-Allow-Origin", "*")
        
        handler.ServeHTTP(w, r)
    })
    
    mux := http.NewServeMux()
    mux.Handle("/metrics", wrappedHandler)
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })
    
    return &http.Server{
        Addr:         ":8080",
        Handler:      mux,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
    }
}

func main() {
    storage := metricsstorage.NewMetricStorage("app")
    
    server := setupMetricsServer(storage)
    
    log.Println("Starting metrics server on :8080")
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Metrics server failed: %v", err)
    }
}

func updateApplicationMetrics(s metricsstorage.Storage) {
    // This function is called on every metrics scrape
    s.GaugeSet("app_current_timestamp", float64(time.Now().Unix()), nil)
    // Add other dynamic metrics here
}
```

## Advanced Usage Patterns

### Thread-Safe Operations

All metric operations are thread-safe and can be used concurrently:

```go
import "sync"

func concurrentMetrics(storage *metricsstorage.MetricStorage) {
    var wg sync.WaitGroup
    
    // Launch multiple goroutines updating metrics
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            storage.CounterAdd("worker_tasks_total", 1.0, map[string]string{
                "worker_id": fmt.Sprintf("%d", id),
                "status":    "completed",
            })
            
            storage.GaugeSet("worker_active", 1.0, map[string]string{
                "worker_id": fmt.Sprintf("%d", id),
            })
        }(i)
    }
    
    wg.Wait()
}
```

### Label Management

The package provides utilities for working with labels:

```go
import "github.com/deckhouse/deckhouse/pkg/metrics-storage/labels"

// Merge multiple label maps
baseLabels := map[string]string{"service": "api", "version": "v1"}
requestLabels := map[string]string{"method": "GET", "endpoint": "/users"}
allLabels := labels.MergeLabels(baseLabels, requestLabels)

// Get sorted label names
labelNames := labels.LabelNames(allLabels)

// Extract label values in specific order
values := labels.LabelValues(allLabels, labelNames)
```

### Error Handling

The storage handles errors gracefully and logs them when using direct collector access:

```go
// Registration errors are returned
counter, err := storage.RegisterCounter("duplicate_metric", []string{"label1"})
if err != nil {
    log.Printf("Failed to register counter: %v", err)
}

// Direct metric operations log errors internally but don't fail
storage.CounterAdd("nonexistent_metric", 1.0, map[string]string{
    "label1": "value1",
})
// This will create the metric automatically if it doesn't exist
```

### Custom Metric Options

Use registration options for advanced metric configuration:

```go
import "github.com/deckhouse/deckhouse/pkg/metrics-storage/options"

// Register with comprehensive options
counter, err := storage.RegisterCounter("api_requests_total",
    []string{"method", "endpoint", "status"},
    options.WithHelp("Total number of API requests processed"),
    options.WithConstantLabels(map[string]string{
        "service": "user-api",
        "version": "1.2.3",
    }),
)

gauge, err := storage.RegisterGauge("database_connections",
    []string{"pool", "status"},
    options.WithHelp("Current number of database connections"),
    options.WithConstantLabels(map[string]string{
        "database": "postgres",
    }),
)

histogram, err := storage.RegisterHistogram("request_size_bytes",
    []string{"content_type", "compressed"},
    []float64{100, 1000, 10000, 100000, 1000000, 10000000},
    options.WithHelp("Distribution of HTTP request sizes"),
    options.WithConstantLabels(map[string]string{
        "direction": "inbound",
    }),
)
```

## Package Structure

The metrics-storage package is organized into several subpackages:

- **`collectors/`**: Type-safe metric collectors (Counter, Gauge, Histogram) with thread-safe value management
- **`labels/`**: Label manipulation utilities including merging, sorting, and validation functions
- **`operation/`**: Batch operation system and action definitions with comprehensive validation
- **`options/`**: Configuration options for storage and metric registration with extensible patterns
- **`storage/`**: Grouped metrics storage implementation with automatic lifecycle management

## API Reference

### Core Interfaces

#### Storage Interface
```go
type Storage interface {
    Registerer
    Collector
    
    ApplyOperation(op operation.MetricOperation, commonLabels map[string]string) error
    ApplyBatchOperations(ops []operation.MetricOperation, labels map[string]string) error
    
    Grouped() GroupedStorage
    Collector() prometheus.Collector
    Handler() http.Handler
}
```

#### GroupedStorage Interface
```go
type GroupedStorage interface {
    Registerer
    GroupedCollector
    
    Collector() prometheus.Collector
    ExpireGroupMetrics(group string)
    ExpireGroupMetricByName(group, name string)
}
```

### Metric Actions

The operation package defines the following actions:

```go
const (
    ActionCounterAdd       // Increment counter by value
    ActionGaugeAdd         // Add value to gauge (can be negative)
    ActionGaugeSet         // Set gauge to specific value
    ActionHistogramObserve // Record histogram observation
    ActionExpireMetrics    // Expire all metrics in a group
)
```

## Best Practices

### 1. Use Prefixes Consistently

```go
// Good: Use a consistent prefix for your application
storage := metricsstorage.NewMetricStorage("myapp")

// Use template variables for dynamic prefixes
storage.CounterAdd("{PREFIX}_requests_total", 1.0, labels)
```

### 2. Group Related Metrics

```go
// Good: Group metrics that have related lifecycles
grouped := storage.Grouped()
grouped.GaugeSet("user_session", "active_count", 150.0, labels)
grouped.GaugeSet("user_session", "avg_duration", 1800.0, labels)

// Later expire the entire group
grouped.ExpireGroupMetrics("user_session")
```

### 3. Use Batch Operations for Efficiency

```go
// Good: Batch multiple operations
operations := []operation.MetricOperation{
    {Name: "metric1", Action: operation.ActionGaugeSet, Value: ptr.To(1.0)},
    {Name: "metric2", Action: operation.ActionCounterAdd, Value: ptr.To(2.0)},
}
storage.ApplyBatchOperations(operations, commonLabels)

// Avoid: Multiple individual operations
storage.GaugeSet("metric1", 1.0, labels)
storage.CounterAdd("metric2", 2.0, labels)
```

### 4. Register Metrics Early

```go
// Good: Register metrics during initialization
func initMetrics(storage *metricsstorage.MetricStorage) {
    storage.RegisterCounter("requests_total", []string{"method", "status"})
    storage.RegisterGauge("active_connections", []string{"pool"})
}

// Then use them throughout the application
storage.CounterAdd("requests_total", 1.0, labels)
```

### 5. Handle Errors Appropriately

```go
// Good: Handle registration errors
counter, err := storage.RegisterCounter("api_calls", []string{"method"})
if err != nil {
    log.Fatalf("Failed to register counter: %v", err)
}

// Good: Validate operations before applying
err := storage.ApplyOperation(op, commonLabels)
if err != nil {
    log.Printf("Failed to apply operation: %v", err)
}
```

### 6. Use Nil-Safe Operations

```go
// The storage handles nil receivers gracefully
var storage *metricsstorage.MetricStorage // nil
storage.CounterAdd("safe_metric", 1.0, nil) // Won't panic

// But ApplyOperation with nil storage returns an error
err := storage.ApplyOperation(op, nil)
// err will be "metric storage is nil"
```

## Common Patterns and Use Cases

### Real-time Metrics Updates

```go
// For applications that need to update metrics frequently
storage := metricsstorage.NewMetricStorage("app", metricsstorage.WithNewRegistry())

// Register metrics once during startup
requestCounter, _ := storage.RegisterCounter("requests_total", 
    []string{"method", "endpoint", "status"})
responseTimeHist, _ := storage.RegisterHistogram("response_time_seconds",
    []string{"endpoint"}, 
    []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0})

// Use throughout the application
func handleRequest(method, endpoint string) {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        responseTimeHist.Observe("", duration, map[string]string{
            "endpoint": endpoint,
        })
    }()
    
    // ... handle request ...
    
    requestCounter.Add("", 1.0, map[string]string{
        "method":   method,
        "endpoint": endpoint,
        "status":   "200",
    })
}
```

### Periodic Metrics Collection

```go
// For scheduled metric updates (like system monitoring)
func updateSystemMetrics(storage *metricsstorage.MetricStorage) {
    operations := []operation.MetricOperation{
        {
            Name:   "memory_usage_bytes",
            Action: operation.ActionGaugeSet,
            Value:  ptr.To(float64(getMemoryUsage())),
            Labels: map[string]string{"component": "system"},
        },
        {
            Name:   "cpu_usage_percent",
            Action: operation.ActionGaugeSet,
            Value:  ptr.To(getCPUUsage()),
            Labels: map[string]string{"component": "system"},
        },
    }
    
    storage.ApplyBatchOperations(operations, map[string]string{
        "hostname": getHostname(),
    })
}

// Run periodically
ticker := time.NewTicker(30 * time.Second)
go func() {
    for range ticker.C {
        updateSystemMetrics(storage)
    }
}()
```

### Dynamic Metric Groups

```go
// For metrics that need coordinated lifecycle management
func trackUserSession(storage *metricsstorage.MetricStorage, userID string) {
    grouped := storage.Grouped()
    groupName := fmt.Sprintf("user_%s", userID)
    
    // Add session metrics to user-specific group
    grouped.GaugeSet(groupName, "session_start_time", 
        float64(time.Now().Unix()), 
        map[string]string{"user_id": userID})
    
    grouped.CounterAdd(groupName, "actions_total", 1.0,
        map[string]string{
            "user_id": userID,
            "action":  "login",
        })
}

func endUserSession(storage *metricsstorage.MetricStorage, userID string) {
    grouped := storage.Grouped()
    groupName := fmt.Sprintf("user_%s", userID)
    
    // Expire all metrics for this user
    grouped.ExpireGroupMetrics(groupName)
}
```

## Migration Guide

### From Direct Prometheus Usage

If migrating from direct Prometheus usage:

```go
// Old Prometheus approach
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "app_requests_total",
            Help: "Total requests",
        },
        []string{"method"},
    )
)

func init() {
    prometheus.MustRegister(requestsTotal)
}

func handleRequest() {
    requestsTotal.WithLabelValues("GET").Inc()
}

// New metrics-storage approach  
storage := metricsstorage.NewMetricStorage("app")

func handleRequest() {
    storage.CounterAdd("requests_total", 1.0, map[string]string{
        "method": "GET",
    })
}
```

### From Other Metrics Libraries

```go
// From other libraries, the pattern is similar:
// 1. Create MetricStorage with appropriate prefix
storage := metricsstorage.NewMetricStorage("myapp")

// 2. Replace direct metric calls with storage methods
// Old: myCounter.Inc()
// New: storage.CounterAdd("my_counter", 1.0, labels)

// Old: myGauge.Set(42.0)  
// New: storage.GaugeSet("my_gauge", 42.0, labels)

// Old: myHistogram.Observe(0.5)
// New: storage.HistogramObserve("my_histogram", 0.5, labels, buckets)
```

## Troubleshooting

### Common Issues

#### Metric Name Conflicts
```go
// Problem: Registering the same metric with different label sets
storage.RegisterCounter("requests", []string{"method"})
storage.RegisterCounter("requests", []string{"endpoint"}) // Error!

// Solution: Use different metric names or ensure label consistency
storage.RegisterCounter("http_requests", []string{"method"})
storage.RegisterCounter("api_requests", []string{"endpoint"})
```

#### Missing Buckets for Histograms
```go
// Problem: Using HistogramObserve without buckets
storage.HistogramObserve("latency", 0.5, labels, nil) // May not work as expected

// Solution: Always provide appropriate buckets
buckets := []float64{0.01, 0.1, 1.0, 10.0}
storage.HistogramObserve("latency", 0.5, labels, buckets)
```

#### Group Expiration Issues
```go
// Problem: Metrics disappearing unexpectedly
grouped := storage.Grouped()
grouped.GaugeSet("temp_group", "metric1", 1.0, labels)
// Later, all metrics in "temp_group" expire when new operations are applied

// Solution: Use consistent group names and understand expiration behavior
grouped.GaugeSet("persistent_group", "metric1", 1.0, labels)
// Only expires when explicitly calling ExpireGroupMetrics("persistent_group")
```

### Performance Considerations

1. **Batch Operations**: Use `ApplyBatchOperations` for multiple metric updates
2. **Label Cardinality**: Keep label combinations reasonable to avoid memory issues
3. **Registration**: Register metrics early and reuse collectors when possible
4. **Group Management**: Use groups judiciously to avoid unnecessary metric churn
