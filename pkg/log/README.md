# pkg/log

Structured logging for Deckhouse components, built on Go's `log/slog`.

Provides a consistent logging interface with named loggers, automatic stack
traces on errors, JSON and plain-text output, and dynamic level control --
all safe for concurrent use.

## Features

- **Built on `log/slog`** -- full API compatibility with Go's standard
  structured logger; any code that accepts `*slog.Logger` works unchanged.
- **Concurrency-safe** -- a single logger can be shared across goroutines
  without external synchronisation.
- **Named loggers** -- create component-scoped loggers that nest names with
  dots (`controller.deployment`).
- **Six log levels** -- Trace, Debug, Info, Warn, Error, Fatal -- extending
  `slog.Level` with a Trace level below Debug and a Fatal level that calls
  `os.Exit(1)`.
- **Dynamic level control** -- change the minimum level at runtime without
  restarting the process.
- **Automatic stack traces** -- Error and Fatal calls capture a goroutine
  stack trace and attach it to the record as structured JSON.
- **Source location** -- file and line are included automatically at Debug
  and Trace levels.
- **Custom encoders with pooled buffers** -- JSON and text formats are
  rendered by hand (no `encoding/json` on the hot path) using a
  `sync.Pool`-backed byte buffer, minimising allocations.
- **Pre-bound attributes and groups** -- attach persistent fields with
  `With` or namespace them with `WithGroup`.
- **Raw JSON / YAML embedding** -- embed parsed JSON or YAML structures
  directly into log records via `RawJSON` / `RawYAML`.
- **Dynamic output switching** -- redirect a logger's writer at runtime
  (e.g. after daemonizing) with `SetOutput`.
- **Nil-safe error helper** -- `Err(err)` handles nil interface pointers
  without panicking.
- **IDE integration** -- set `IDEA_DEVELOPMENT` to get padded source paths
  for clickable links in JetBrains IDEs.

## Usage

### Global logger

The package exposes top-level functions that delegate to a shared default
logger. For simple programs and CLI tools this is the quickest way to start:

```go
log.Info("Listening", slog.String("addr", ":8080"))
log.Error("Startup failed", log.Err(err))
```

Every level has a `*Context` variant that forwards a `context.Context`:

```go
log.InfoContext(ctx, "Request served", slog.Int("status", 200))
```

`Error`, `ErrorContext`, `Fatal`, and `FatalContext` automatically capture a
goroutine stack trace and attach it to the log record.

### Creating loggers

Use `NewLogger` with functional options:

```go
logger := log.NewLogger(
    log.WithLevel(slog.LevelDebug),
    log.WithOutput(os.Stderr),
    log.WithHandlerType(log.TextHandlerType),
)
```

Available options:

| Option | Default | Description |
|---|---|---|
| `WithLevel` | `slog.LevelInfo` | Minimum enabled level |
| `WithOutput` | `os.Stdout` | Destination `io.Writer` |
| `WithHandlerType` | `JSONHandlerType` | `JSONHandlerType` or `TextHandlerType` |
| `WithTimeFunc` | identity | Transform timestamps before formatting |

### Levels

Six levels are defined, extending `slog.Level`:

| Level | Value | Behaviour |
|---|---|---|
| `LevelTrace` | -8 | Most verbose |
| `LevelDebug` | -4 | Source location enabled by default |
| `LevelInfo` | 0 | General operational messages |
| `LevelWarn` | 4 | Warnings |
| `LevelError` | 8 | Errors -- stack trace attached automatically |
| `LevelFatal` | 12 | Errors -- stack trace + `os.Exit(1)` |

Levels can be changed at runtime without restarting:

```go
logger.SetLevel(log.LevelWarn)
log.SetDefaultLevel(log.LevelDebug)
```

Parse from strings (e.g. environment variables, flags):

```go
lvl, err := log.ParseLevel("debug")     // returns (LevelDebug, nil)
lvl := log.LogLevelFromStr("unknown")   // defaults to LevelInfo
```

### Named loggers

Create component-scoped loggers. Names are joined with dots:

```go
ctrl := logger.Named("controller")
deploy := ctrl.Named("deployment")   // name: "controller.deployment"

deploy.Info("Scaled", slog.Int("replicas", 3))
// JSON: {"level":"info","logger":"controller.deployment","msg":"Scaled","replicas":3,"time":"..."}
```

### Pre-bound attributes and groups

Attach persistent fields with `With`, or namespace them with `WithGroup`:

```go
reqLogger := logger.With(
    slog.String("request_id", id),
    slog.String("remote", r.RemoteAddr),
)
reqLogger.Info("Handling request")

httpLogger := logger.WithGroup("http")
httpLogger.Info("Response",
    slog.String("method", "GET"),
    slog.Int("status", 200),
)
// JSON: {"level":"info","msg":"Response","http":{"method":"GET","status":200},"time":"..."}
```

### Raw data logging

Embed parsed JSON or YAML structures directly into log records:

```go
logger.Info("Config loaded",
    log.RawJSON("config", `{"debug":true,"timeout":30}`))

logger.Info("Manifest applied",
    log.RawYAML("spec", "replicas: 3\nstrategy: RollingUpdate"))
```

If parsing fails the original text is logged as a plain string.

### Error helpers

```go
log.Err(err)                  // slog.Attr with key "error"
log.Type("obj", myStruct)     // slog.Attr with key "obj", value = type name
```

## Output formats

### JSON (default)

```json
{"level":"info","logger":"controller","msg":"Scaled","replicas":3,"time":"2024-01-15T10:30:00Z"}
```

Field order: `level`, `logger`, `msg`, `source`, user fields, `stacktrace`, `time`.

### Text

```
2024-01-15T10:30:00Z INFO logger=controller msg='Scaled' replicas='3'
```

## Global logger management

```go
log.SetDefault(myLogger)                 // replace the global logger
log.SetDefaultLevel(log.LevelDebug)      // change its level
log.Default()                            // retrieve it
```

## Dynamic output switching

Redirect a logger's output at runtime (e.g. after daemonizing):

```go
logger.SetOutput(logFile)
```

## Source location

Source file and line are included automatically at Debug and Trace levels.
### IDE Integration

Set the `IDEA_DEVELOPMENT` environment variable to pad source paths for
clickable links in JetBrains IDEs.

## Testing

```go
nop := log.NewNop()                        // discard all output

var buf bytes.Buffer
tl := log.NewLogger(
    log.WithOutput(&buf),
    log.WithTimeFunc(func(_ time.Time) time.Time {
        return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    }),
)
// buf now contains deterministic output for snapshot tests
```

## Best practices

### Prefer named loggers over the global logger

The global (package-level) functions are convenient for small programs, but
in larger codebases every component should create its own named logger.
This makes it trivial to filter logs by source:

```go
func NewReconciler(logger *log.Logger) *Reconciler {
    return &Reconciler{
        log: logger.Named("reconciler"),
    }
}
```

### Pre-bind common fields with `With`

If a set of attributes appears on every log call in a function or handler,
bind them once and reuse the enriched logger. This reduces boilerplate and
lowers per-call allocation cost:

```go
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
    reqLog := s.log.With(
        slog.String("method", r.Method),
        slog.String("path", r.URL.Path),
        slog.String("remote", r.RemoteAddr),
    )

    reqLog.Info("Request received")
    // ... later ...
    reqLog.Info("Response sent", slog.Int("status", code))
}
```

### Namespace fields with `WithGroup` to prevent key collisions

When different subsystems add attributes with generic names (`id`,
`status`), wrapping each in a group keeps the output unambiguous:

```go
httpLog := logger.WithGroup("http")
dbLog   := logger.WithGroup("db")
```

### Use `LogAttrs` in hot paths

`LogAttrs` accepts `slog.Attr` values directly, avoiding the `any`
interface boxing that the variadic `Info`/`Debug`/... methods require.
In tight loops this can measurably reduce allocations:

```go
logger.LogAttrs(ctx, slog.LevelInfo, "request handled",
    slog.String("method", "GET"),
    slog.Int("status", 200),
)
```

### Set the level as high as possible

Disabled-level calls short-circuit before allocating a record or
formatting any attributes. Running at `Info` in production and switching
to `Debug` only when investigating issues is the normal pattern:

```go
lvl := log.LogLevelFromStr(os.Getenv("LOG_LEVEL"))   // defaults to Info
logger := log.NewLogger(log.WithLevel(lvl.Level()))
```

### Use `Err` instead of manually stringifying errors

`log.Err(err)` is nil-safe -- it will not panic if `err` is a nil
interface wrapping a non-nil typed pointer. Always prefer it over
`slog.String("error", err.Error())`:

```go
if err := doWork(); err != nil {
    logger.Error("Work failed", log.Err(err))
}
```

### Reserve `Fatal` for top-level commands

`Fatal` calls `os.Exit(1)`, which skips deferred functions and does not
unwind the stack. Use it only in `main()` or CLI entry points; libraries
should return errors instead.

### Freeze time in tests

Use `WithTimeFunc` to make timestamps deterministic so log output can be
compared with golden files or exact string assertions:

```go
logger := log.NewLogger(
    log.WithOutput(&buf),
    log.WithTimeFunc(func(_ time.Time) time.Time {
        return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    }),
)
```
