# dhctl provider validator protocol

This document specifies the protocol between dhctl and a cloud provider's validator
binary. The Go implementation of both sides lives in this module; a validator written
in another language implements the same gRPC service.

## Overview

The validator is a standalone executable shipped in the provider's OCI bundle. dhctl
starts it as a subprocess, calls it over gRPC on a unix socket, and stops it when the
call is done. There is one action today — `Validate`, run before dhctl touches any
infrastructure — and the protocol is shaped so more can be added without breaking
existing binaries.

## Binary location

The binary must be named `validator` and live in a provider-named directory inside
dhctl's download root:

```
<download-root>/
  dvp/
    validator        ← this binary
```

The provider's terraform-manager image is unpacked into the download root, so a
binary shipped at `/validator` inside that image lands at
`<download-root>/<provider-name>/validator` on disk. dhctl discovers it by that path
and requires the execute bit.

## Invocation

dhctl invokes the binary with the socket it has created:

```
validator serve --address=<unix socket path>
```

The validator listens on that socket and serves until it is stopped. Notes that
matter in practice:

- **Socket path length.** A unix socket path is capped at 104 bytes on darwin and
  108 on Linux; over the limit, `bind` fails with `invalid argument`. dhctl allocates
  a short path.
- **Readiness.** The validator does not announce readiness and serves no health
  service. The caller waits for the socket file to appear, or calls with
  `grpc.WaitForReady(true)` and a context deadline — `WaitForReady` waits
  indefinitely on its own. Either way the caller must also watch the process: one
  that exits early (an old binary that does not know `serve`) has to fail the
  operation rather than wait out the deadline.
- **Shutdown.** dhctl sends `SIGTERM` and the validator stops gracefully. A binary
  built with this module's `server` package installs no signal handler of its own; it
  waits for the signal itself and calls `Stop`.
- **stdout/stderr** belong to the validator. They carry diagnostics only — no part of
  the protocol travels through them.

## Transport

gRPC over the unix socket, no TLS (the socket's file permissions are the boundary).

Message limit: **8 MiB** in each direction, applied by both sides. gRPC's 4 MiB
default is too small — the payload carries every NodeGroup, InstanceClass and
credential Secret of a cluster.

## Service: validate

`dhctl.provider.validate.v1.ValidateService`, defined in
[`api/pb/validate/v1/validate.proto`](api/pb/validate/v1/validate.proto). The Go side —
generated types, the payload, the result and the conversions — is
[`api/validate/v1`](api/validate/v1).

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
on both sides. The Go definition is `validatev1.Input` in [`validate`](validate).

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
| `vars` | object | Structured provider data dhctl collected: module `settings` (the full ModuleConfig object), `nodeGroups`, `instanceClasses`, credential `secrets`. Absent when there was nothing to collect — a validator must tolerate its absence |

`operation` is required, and a value outside the three above is rejected with
`InvalidArgument`: a validator decides what to check from this field, and an unknown
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

An empty response means the configuration is valid. `errors` block the caller's
operation; `warnings` do not. Violations arrive in the order the validator recorded
them, and that order is what a caller prints.

`value` is display-only and already rendered as a string. A validator reporting a
sensitive field sends a placeholder such as `"masked"` instead of the value.

### Statuses

A failure of the validator itself is never a violation — it is a gRPC status:

| Code | Meaning |
|---|---|
| `OK` | The validator ran. The response says whether the configuration is valid |
| `InvalidArgument` | The request was malformed or its `operation` was rejected |
| `Unimplemented` | The validator does not serve this action |
| `Internal` | The validator failed or panicked. A panic carries its stack in the message |

The caller must fail closed: any status other than `OK`, and any response it cannot read,
blocks the operation instead of counting as "validated".

## Implementing a validator in Go

Import [`api/validate/v1`](api/validate/v1) and [`server`](server); see the
runnable example in the `server` package documentation.

```go
type myValidator struct{}

func (myValidator) Validate(_ context.Context, input validatev1.Input) (validatev1.Output, error) {
	var output validatev1.Output
	if input.CloudProviderVars == nil || len(input.CloudProviderVars.Secrets) == 0 {
		output.AddError("Secret/d8-credentials", "credential_secret_required", nil,
			`credential Secret "d8-credentials" is required`)
	}
	return output, nil
}

func main() {
	address := flag.String("address", "", "unix socket to serve on")
	flag.Parse()

	ctx, unhook := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer unhook()

	service := server.NewValidateService(myValidator{})

	validator, err := server.Start(server.Config{Address: *address}, service)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	<-ctx.Done() // SIGTERM from dhctl

	if err := validator.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

The `Output` and the `error` mean different things, and the split is the whole point
of the contract: an `Output` carries what is wrong with the configuration — the check
working correctly — while an `error` means the check could not be made. Panics are
recovered and reported as `Internal` with the stack, so a bug surfaces instead of
looking like a process that died mid-call.

## Calling a validator in Go

Import [`api/validate/v1`](api/validate/v1) and [`client`](client). The caller
owns the process, the socket and the deadlines:

```go
conn, err := grpc.NewClient("unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
// …
defer conn.Close()

output, err := client.NewClient(conn, client.Config{
	GRPCOptions: []grpc.CallOption{grpc.WaitForReady(true)},
}).Validate(ctx, input)
if err != nil {
	return err // the validator failed
}
if len(output.Errors) > 0 {
	return errors.New(output.Errors.String()) // the configuration is wrong
}
return nil
```

## Adding an action

Each action is its own service in [`api/pb`](api/pb), its own package in
[`api`](api) and its own `server.Service`
constructor, so a binary that does not implement one simply does not pass it to
`server.Start` and a caller sees `Unimplemented`. The full recipe is in the module
documentation ([`doc.go`](doc.go)).
