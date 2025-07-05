# log

A flexible and structured logging package for Golang applications in the Deckhouse ecosystem.

## Overview

The `log` package provides a consistent logging interface for Deckhouse components. It builds upon Go's `log/slog` package to offer structured logging capabilities with additional features tailored for Kubernetes and cloud-native applications.

## Features

- Structured logging with key-value pairs
- Multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
- Context-aware logging
- Customizable output formats (JSON, text)
- Integration with Kubernetes logging standards
- Context propagation for distributed tracing

## Installation

```bash
go get github.com/deckhouse/deckhouse/pkg/log
```

## Basic Usage

```go
package main

import (
    "context"
    
    "github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
    // Get the default logger
    logger := log.Logger
    
    // Basic logging
    logger.Info("Application started")
    
    // Logging with key-value fields
    logger.Info("Processing request", 
        "requestID", "abc-123",
        "method", "GET",
        "path", "/api/v1/users",
    )
    
    // Logging with key-value slog attributes
    logger.Info("Processing request", 
        slog.String("requestID", "abc-123"),
        slog.String("method", "GET"),
        slog.String("path", "/api/v1/users"),
    )
    
    // Error logging
    err := someFunction()
    if err != nil {
        logger.Error("Failed to execute function", "error", err)
    }
    
    // Debug logging
    logger.Debug("Detailed operation information", 
        slog.String("operation", "database query"),
        slog.Uint64("duration_ms", 42),
    )
}
```

## Creating Custom Loggers

```go
package main

import (
    "os"
    "log/slog"
    
    "github.com/deckhouse/deckhouse/pkg/log"
)

func main() {
    // Create a logger with custom options
    customLogger := log.NewLogger(
        log.WithOutput(os.Stdout),
        log.WithLevel(slog.LevelDebug),
        log.WithHandlerType(log.JSONHandlerType),
    )
    
    // Use the custom logger
    customLogger.Info("Using custom logger")
    
    // Create a logger for a specific component
    moduleLogger := log.NewLogger().Named("logger-name")
    
    moduleLogger.Info("Controller initialized")
}
```

## Log Levels

```go
// Set global log level
log.SetDefaultLevel(log.LevelDebug)

// use global logger in variable
logger:= log.Default()

// Log at specific levels
logger.Debug("Debug information")
logger.Info("Informational message")
logger.Warn("Warning message")
logger.Error("Error message")
logger.Fatal("Fatal message")

// Check if a level would be logged
if logger.Enabled(context.TODO(), log.LevelDebug.Level()) {
    // Only prepare expensive debug data if it would be logged
    debugData := prepareExpensiveDebugData()
    logger.Debug("Expensive debug info", "data", debugData)
}
```

## Best Practices

1. **Use structured logging**: Always use key-value pairs instead of formatting strings
   ```go
   // Good
   logger.Info("User created", slog.Uint64("userID", user.ID), slog.String("role", user.Role))
   
   // Avoid
   logger.Info(fmt.Sprintf("User created: ID=%d, Role=%s", user.ID, user.Role))
   ```

2. **Use appropriate log levels**:
   - `Debug`: Detailed information, useful for development and troubleshooting
   - `Info`: Confirmation that things are working as expected
   - `Warn`: Something unexpected happened, but the application can continue
   - `Error`: Something failed, but the application can still function

3. **Include context in logs**:
   ```go
   logger.Info("Processing complete", 
       slog.String("operation", "sync"),
       slog.String("resourceType", "deployment"),
       slog.String("namespace", "default"),
       slog.String("name", "my-app"),
       slog.Uint64("duration_ms", 235),
   )
   ```

4. **Use with-style methods for adding context to loggers**:
   ```go
   controllerLogger := logger.With(
       slog.String("controller", "deployment"),
       slog.String("namespace", deployment.Namespace),
   )
   ```

## License

Apache 2.0