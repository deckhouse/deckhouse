# dhctl provider plugin protocol

This document specifies the protocol between a host (dhctl) and a cloud provider
plugin binary. The Go implementation of both sides lives in this module; a plugin in
another language implements the same gRPC service.

## Overview

A plugin is a standalone executable shipped in the provider's OCI bundle. The host
starts it as a subprocess, calls it over gRPC on a unix socket, and stops it when the
call is done. There is one action today — `Validate`, run before the host touches any
infrastructure — and the protocol is shaped so more can be added without breaking
existing plugins.

## Binary location

The binary must be named `validator` and live in a provider-named directory inside
the host's download root:

```
<download-root>/
  dvp/
    validator        ← this binary
```

The provider's terraform-manager image is unpacked into the download root, so a
plugin shipped at `/validator` inside that image lands at
`<download-root>/<provider-name>/validator` on disk. The host discovers it by that
path and requires the execute bit.

## Invocation

The host invokes the binary with the socket it has created:

```
validator serve --address=<unix socket path>
```

The plugin listens on that socket and serves until it is stopped. Notes that matter
in practice:

- **Socket path length.** A unix socket path is capped at 104 bytes on darwin and
  108 on Linux; over the limit, `bind` fails with `invalid argument`. The host
  allocates a short path.
- **Readiness.** The plugin does not announce readiness. A host that wants the first
  call to wait for the socket uses `grpc.WaitForReady(true)` together with a context
  deadline — `WaitForReady` waits indefinitely on its own.
- **Shutdown.** The host sends `SIGTERM` and the plugin stops gracefully. A plugin
  built with this module's `server` package installs no signal handler of its own;
  the binary wires signals to the context it passes to `Serve`.
- **stdout/stderr** are the plugin's own. They carry diagnostics only — no part of
  the protocol travels through them.

## Transport

gRPC over the unix socket, no TLS (the socket's file permissions are the boundary).

Message limit: **8 MiB** in each direction, applied by both sides. gRPC's 4 MiB
default is too small — the payload carries every NodeGroup, InstanceClass and
credential Secret of a cluster.

## Service: validate

`dhctl.provider.validate.v1.ValidateService`, defined in
[`api/pb/validate/v1/validate.proto`](api/pb/validate/v1/validate.proto).

```proto
service ValidateService {
  rpc Validate(ValidateRequest) returns (ValidateResponse);
}
```

### Request

```proto
message ValidateRequest {
  bytes input_json = 1;
}
```

`input_json` is a JSON object. It stays JSON rather than becoming protobuf messages
because every field that carries substance is an arbitrary Kubernetes object — a
`google.protobuf.Struct` would be exactly as untyped while costing a conversion layer
on both sides. The Go definition is `validate.Input` in [`validate`](validate).

```json
{
  "providerName": "dvp",
  "clusterPrefix": "my-cluster",
  "layout": "Standard",
  "operation": "bootstrap",
  "providerClusterConfiguration": { },
  "vars": {
    "settings": { },
    "nodeGroups": { "master": { } },
    "instanceClasses": { "worker": { } },
    "secrets": { "d8-credentials": { } }
  }
}
```

| Field | Type | Description |
|---|---|---|
| `providerName` | string | Provider identifier, e.g. `"dvp"` |
| `clusterPrefix` | string | Prefix applied to cloud resource names |
| `layout` | string | Provider layout name |
| `operation` | string | One of `"bootstrap"`, `"converge"`, `"destroy"` |
| `providerClusterConfiguration` | object | Parsed `providerClusterConfiguration` section |
| `vars` | object | Structured provider data the host collected: module `settings` (the full ModuleConfig object), `nodeGroups`, `instanceClasses`, credential `secrets`. Absent when there was nothing to collect — a plugin must tolerate its absence |

`operation` is required, and a value outside the three above is rejected with
`InvalidArgument`: a plugin decides what to check from this field, and an unknown
value would silently get the checks of some other phase.

`vars.secrets` holds credential Secrets — objects of type
`cloud-provider.deckhouse.io/credentials`. The payload therefore carries credentials:
neither side may log it, attach it to traces, or quote it in an error.

### Response

```proto
message ValidateResponse {
  repeated Violation errors = 1;
  repeated Violation warnings = 2;
}

message Violation {
  string path = 1;
  string code = 2;
  string message = 3;
  string value = 4;
}
```

An empty response means the configuration is valid. `errors` block the host's
operation; `warnings` do not. Violations are ordered by `code` then `path` and
deduplicated by that pair, so the text a host prints is stable across runs.

`value` is display-only and already rendered as a string. A plugin reporting a
sensitive field sends a placeholder such as `"masked"` instead of the value.

### Statuses

A failure of the plugin itself is never a violation — it is a gRPC status:

| Code | Meaning |
|---|---|
| `OK` | The plugin ran. The response says whether the configuration is valid |
| `InvalidArgument` | The request was malformed or its `operation` was rejected |
| `Unimplemented` | The plugin does not serve this action |
| `Internal` | The plugin failed or panicked. A panic carries its stack in the message |

A host must fail closed: any status other than `OK`, and any response it cannot read,
blocks the operation instead of counting as "validated".

## Implementing a plugin in Go

Import [`validate`](validate) and [`server`](server); see the runnable
example in the `server` package documentation.

```go
type myValidator struct{}

func (myValidator) Validate(_ context.Context, input validate.Input) (validate.Result, error) {
	var result validate.Result
	if input.CloudProviderVars == nil || len(input.CloudProviderVars.Secrets) == 0 {
		result.AddError("Secret/d8-credentials", "credential_secret_required", nil,
			`credential Secret "d8-credentials" is required`)
	}
	return result, nil
}

func main() {
	address := flag.String("address", "", "unix socket to serve on")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	handlers := server.Handlers{Validator: myValidator{}}
	if err := server.Serve(ctx, "unix", *address, handlers); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

The `Result` and the `error` mean different things, and the split is the whole point
of the contract: a `Result` carries what is wrong with the configuration — the plugin
working correctly — while an `error` means the plugin could not decide. Panics are
recovered and reported as `Internal` with the stack, so a bug in a plugin surfaces
instead of looking like a process that died mid-call.

## Calling a plugin in Go

Import [`validate`](validate) and [`client`](client). The host owns the
process, the socket and the deadlines:

```go
conn, err := grpc.NewClient("unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
// …
defer conn.Close()

result, err := client.NewClient(conn, client.WithCallOptions(grpc.WaitForReady(true))).
	Validate(ctx, input)
if err != nil {
	return err // the plugin failed
}
return result.ErrorOrNil() // the configuration is wrong
```

## Adding an action

Each action is its own service in its own `api/pb/<action>/v1` package, so a plugin that
does not implement one simply does not register it and a caller sees `Unimplemented`.
The full recipe is in the module documentation ([`doc.go`](doc.go)).
