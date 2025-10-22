# log

A flexible and structured logging package for Golang applications in the Deckhouse ecosystem.

## Overview

The `log` package provides a consistent logging interface for Deckhouse components. It builds upon Go's `log/slog` package to offer structured logging capabilities with additional features tailored for Kubernetes and cloud-native applications.

## Features

- **Structured Logging**: Built on top of Go's `log/slog` with key-value pairs
- **Multiple Log Levels**: TRACE, DEBUG, INFO, WARN, ERROR, FATAL
- **Context-Aware Logging**: Automatic source file location and stack traces for errors
- **Multiple Output Formats**: JSON and Text handlers with customizable formatting
- **Named Loggers**: Hierarchical logger naming with dot-separated components
- **Stack Trace Support**: Automatic stack trace capture for error and fatal levels
- **Raw Data Support**: JSON and YAML raw data logging with automatic parsing
- **Thread-Safe**: Concurrent logging support with atomic operations
- **Development Features**: IDE-friendly source formatting and customizable time functions

## Installation

```bash
go get github.com/deckhouse/deckhouse/pkg/log
```

## Quick Start

### Using the Global Logger

```go
package main

import (
    "log/slog"
    "github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
    // Basic logging
    log.Info("Application started")
    
    // Logging with key-value pairs (using slog attributes)
    log.Info("Processing request", 
        slog.String("requestID", "abc-123"),
        slog.String("method", "GET"),
        slog.String("path", "/api/v1/users"),
    )
    
    // Using slog attributes
    log.Info("User action", 
        slog.String("userID", "123"),
        slog.String("action", "login"),
        slog.Duration("duration", time.Second*2),
    )
    
    // Error logging (automatically includes stack trace)
    log.Error("Failed to process request", slog.String("error", err.Error()))
    
    // Context-aware logging
    log.InfoContext(ctx, "Request processed", slog.String("status", "success"))
}
```

### Creating Custom Loggers

```go
package main

import (
    "os"
    "log/slog"
    "github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
    // Create a JSON logger
    jsonLogger := log.NewLogger(
        log.WithOutput(os.Stdout),
        log.WithLevel(slog.LevelDebug),
        log.WithHandlerType(log.JSONHandlerType),
    )
    
    // Create a text logger
    textLogger := log.NewLogger(
        log.WithOutput(os.Stderr),
        log.WithLevel(slog.LevelInfo),
        log.WithHandlerType(log.TextHandlerType),
    )
    
    // Create a no-op logger for testing
    nopLogger := log.NewNop()
    
    jsonLogger.Info("Using JSON logger")
    textLogger.Info("Using text logger")
}
```

## Log Levels

The package provides six log levels with the following hierarchy:

```go
const (
    LevelTrace Level = -8  // Most verbose
    LevelDebug Level = -4  // Debug information
    LevelInfo  Level = 0   // General information
    LevelWarn  Level = 4   // Warning messages
    LevelError Level = 8   // Error messages (includes stack trace)
    LevelFatal Level = 12  // Fatal errors (exits application)
)
```

### Working with Log Levels

```go
// Set global log level
log.SetDefaultLevel(log.LevelDebug)

// Get the default logger
logger := log.Default()

// Log at specific levels
logger.Debug("Debug information")
logger.Info("Informational message")
logger.Warn("Warning message")
logger.Error("Error message") // Automatically includes stack trace
logger.Fatal("Fatal message") // Exits with os.Exit(1)

// Parse level from string
level, err := log.ParseLevel("debug")
if err != nil {
    log.Error("Invalid log level", log.Err(err))
}

// Check if a level would be logged
if logger.Enabled(context.TODO(), log.LevelDebug.Level()) {
    // Only prepare expensive debug data if it would be logged
    debugData := prepareExpensiveDebugData()
    logger.Debug("Expensive debug info", slog.Any("data", debugData))
}
```

## Named Loggers

Create hierarchical loggers with dot-separated names:

```go
// Create named loggers
controllerLogger := log.Default().Named("controller")
deploymentLogger := controllerLogger.Named("deployment")
podLogger := deploymentLogger.Named("pod")

// Results in logger names: "controller", "controller.deployment", "controller.deployment.pod"
controllerLogger.Info("Controller started")
deploymentLogger.Info("Deployment created", slog.String("name", "my-app"))
podLogger.Info("Pod scheduled", slog.String("node", "worker-1"))
```

## Advanced Features

### Raw Data Logging

Log JSON and YAML data with automatic parsing:

```go
// Raw JSON logging
logger.Info("Configuration loaded",
    log.RawJSON("config", `{"debug": true, "timeout": 30}`))

// Raw YAML logging  
logger.Info("Manifest applied",
    log.RawYAML("manifest", `
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
`))
```

### Error Handling Helpers

```go
// Helper functions for common patterns
logger.Info("Type information", log.Type("object", myStruct))
logger.Error("Operation failed", log.Err(err))
```

### Grouping and Context

```go
// Group related fields
groupedLogger := logger.WithGroup("http")
groupedLogger.Info("Request received", 
    slog.String("method", "GET"), 
    slog.String("path", "/api/users"))
// Output: {"level":"info","msg":"Request received","http":{"method":"GET","path":"/api/users"}}

// Add persistent context
contextLogger := logger.With(
    slog.String("service", "user-api"),
    slog.String("version", "1.2.3"))
contextLogger.Info("Service started")
// Output: {"level":"info","msg":"Service started","service":"user-api","version":"1.2.3"}
```

## Output Formats

### JSON Format (Default)

```json
{
  "level": "info",
  "logger": "controller.deployment",
  "msg": "Deployment created",
  "source": "controllers/deployment.go:45",
  "name": "my-app",
  "namespace": "default",
  "time": "2006-01-02T15:04:05Z"
}
```

### Text Format

```
2006-01-02T15:04:05Z INFO logger=controller.deployment msg='Deployment created' source=controllers/deployment.go:45 name='my-app' namespace='default'
```

## Configuration Options

```go
// Available options when creating a logger
logger := log.NewLogger(
    log.WithLevel(slog.LevelDebug),           // Set log level
    log.WithOutput(os.Stdout),                // Set output writer
    log.WithHandlerType(log.JSONHandlerType), // JSON or Text handler
    log.WithTimeFunc(time.Now),               // Custom time function for testing
)
```

## Global Logger Management

```go
// Set a custom logger as the global default
customLogger := log.NewLogger(log.WithLevel(slog.LevelDebug))
log.SetDefault(customLogger)

// Change the global log level
log.SetDefaultLevel(log.LevelError)

// Get the current global logger
currentLogger := log.Default()
```

## Best Practices

1. **Use structured logging**: Always prefer key-value pairs over formatted strings
   ```go
   // Good
   log.Info("User created", 
       slog.Uint64("userID", user.ID), 
       slog.String("role", user.Role))
   
   // Avoid (deprecated)
   log.Infof("User created: ID=%d, Role=%s", user.ID, user.Role)
   ```

2. **Use appropriate log levels**:
   - `Trace`: Very detailed debugging information
   - `Debug`: Detailed information for debugging (enables source location)
   - `Info`: General information about application flow
   - `Warn`: Something unexpected but recoverable happened
   - `Error`: An error occurred but application can continue (includes stack trace)
   - `Fatal`: A fatal error occurred, application will exit

3. **Use named loggers for components**:
   ```go
   controllerLogger := log.Default().Named("deployment-controller")
   controllerLogger.Info("Starting reconciliation", 
       slog.String("namespace", ns), 
       slog.String("name", name))
   ```

4. **Use context-aware logging**:
   ```go
   log.InfoContext(ctx, "Processing request", 
       slog.String("traceID", getTraceID(ctx)),
       slog.String("userID", getUserID(ctx)))
   ```

5. **Leverage persistent context**:
   ```go
   requestLogger := log.Default().With(
       slog.String("requestID", reqID),
       slog.String("userID", userID))
   requestLogger.Info("Request started")
   requestLogger.Info("Request completed")
   ```

## Error Handling and Stack Traces

The package automatically captures stack traces for `Error` and `Fatal` level logs:

```go
// Stack trace is automatically captured
log.Error("Database connection failed", log.Err(err))

// Manual context with stack trace
ctx := context.Background()
log.ErrorContext(ctx, "Critical failure", slog.String("component", "auth"))
```

## Development Features

### IDE Integration

Set the `IDEA_DEVELOPMENT` environment variable to get IDE-friendly source formatting:

```bash
export IDEA_DEVELOPMENT=1
```

This changes the source format from `file.go:123` to ` file.go:123 ` for better IDE integration.

### Testing Support

```go
// Create a logger that discards output for tests
testLogger := log.NewNop()

// Create a logger with custom time for deterministic tests
testLogger := log.NewLogger(
    log.WithTimeFunc(func(t time.Time) time.Time {
        return time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
    }))
```

## Migration from Printf-style Logging

The package provides deprecated printf-style methods for backward compatibility, but structured logging is recommended:

```go
// Deprecated - will be removed
log.Infof("User %s logged in", username)

// Recommended
log.Info("User logged in", slog.String("username", username))
```
