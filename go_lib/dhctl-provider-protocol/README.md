# dhctl provider validator protocol

This document specifies the protocol between dhctl and a cloud provider's validator
binary. The Go implementation of both sides lives in this module; a validator written
in another language implements the same gRPC service.

## Overview

The validator is a standalone executable shipped in the provider's OCI bundle. dhctl
starts it as a subprocess, calls it over gRPC on the address it passes in, and stops
it when the call is done. There is one action today — `Validate`, run before dhctl touches any
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

dhctl invokes the binary with the endpoint it asks for:

```
validator serve --address=<address> [--network=<network>]
```

`--address` is `host:port`, or a socket path when `--network=unix`. dhctl asks for
`127.0.0.1:0`, so the kernel picks the port and the validator is the one that knows it.

- **Announcement.** Once bound, the validator must write the endpoint it got, in the
  form `server.ListeningLine` produces, to stdout or stderr. That line is how the
  caller learns where to dial; a validator built on this module's `server` package
  emits it from `Start`. The caller reads it out of the process output, so it must not
  be suppressed by a log level.
- **Shutdown.** dhctl sends `SIGTERM` and the validator stops gracefully. A binary
  built with this module's `server` package installs no signal handler of its own; it
  waits for the signal itself and calls `Stop`.

## Transport

gRPC over the address dhctl passes in, no TLS: the validator is a subprocess of the
caller and the transport never leaves the host.

Message limit: **8 MiB** in each direction, the default on both sides. gRPC's own
4 MiB is too small — the payload carries every NodeGroup, InstanceClass and credential
Secret of a cluster.

`GRPCOptions` in a `Config` are appended to that default set, so options a caller
passes cannot drop the limits.

## Service: validate

`dhctl.provider.validate.v1.ValidateService`, defined in
[`api/pb/validate/v1/validate.proto`](api/pb/validate/v1/validate.proto). The Go side —
the generated request, response and violation types, the hand-written payload and the
conversions between them — is [`api/validate/v1`](api/validate/v1).

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
on both sides. The Go definition is `Input` in [`api/validate/v1`](api/validate/v1).

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
| `operation` | string | One of `"bootstrap"`, `"converge"`, `"destroy"`; in Go it is the named type `Operation` with a constant per value |
| `providerClusterConfiguration` | object | Parsed `providerClusterConfiguration` section |
| `vars` | object | Structured provider data dhctl collected: module `settings` (the full ModuleConfig object), `nodeGroups`, `instanceClasses`, credential `secrets`. Absent when there was nothing to collect — a validator must tolerate its absence |

`vars.secrets` holds credential Secrets — objects of type
`cloud-provider.deckhouse.io/credentials`. The payload therefore carries credentials:
neither side may log it, attach it to traces, or quote it in an error.

### Response

```proto
message ValidateResponse {
  repeated ViolationResponse errors   = 1;
  repeated ViolationResponse warnings = 2;
}

message ViolationResponse {
  string path = 1;
  string code = 2;
  string message = 3;
  string value = 4;
}
```

An empty response means the configuration is valid. `errors` block the caller's
operation; `warnings` do not. Violations arrive in the order the validator recorded
them, and that order is what a caller prints. Rendering is the caller's: `path`,
`": "` and `message` per line, with the path dropped when empty.

`value` is display-only and already rendered as a string. A validator reporting a
sensitive field sends a placeholder such as `"masked"` instead of the value.

### Statuses

A failure of the validator itself is never a violation — it is a gRPC status:

| Code | Meaning |
|---|---|
| `OK` | The validator ran. The response says whether the configuration is valid |
| `InvalidArgument` | The request did not decode |
| `Unimplemented` | The server registered no service for this action |
| `Internal` | The validator returned an error or panicked |
| `Unavailable` | The server is gone: stopped, or it never came up |

A plain error from a validator becomes `Internal` — the caller cannot tell a broken
dependency from a bug, and either way nothing was validated. A validator that wants a
different code builds its error with
[`status`](https://pkg.go.dev/google.golang.org/grpc/status), and that code is passed
through untouched:

```go
return nil, status.Error(codes.InvalidArgument, "layout is not supported")
```

Wrapping such an error in `fmt.Errorf` is pointless: the status is still found, but
the wrapping text is dropped from the message.

The caller must fail closed: any status other than `OK`, and any response it cannot read,
blocks the operation instead of counting as "validated".

## Implementing a validator in Go

Import [`api/validate/v1`](api/validate/v1) and [`server`](server).

```go
type myValidator struct{}

func (myValidator) Validate(_ context.Context, input validatev1.Input) (*validatev1.ValidateResponse, error) {
	resp := &validatev1.ValidateResponse{}
	if input.CloudProviderVars == nil || len(input.CloudProviderVars.Secrets) == 0 {
		resp.Errors = append(resp.Errors, &validatev1.ViolationResponse{
			Path:    "Secret/d8-credentials",
			Code:    "credential_secret_required",
			Message: `credential Secret "d8-credentials" is required`,
		})
	}
	return resp, nil
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintf(os.Stderr, "usage: %s serve --address=<address> [--network=<network>]\n", os.Args[0])
		os.Exit(2)
	}

	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	address := flags.String("address", "", "address to serve on")
	network := flags.String("network", server.DefaultNetwork, "network to serve on: unix or tcp")

	if err := flags.Parse(os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	validator, err := server.Start(
		server.Config{Network: *network, Address: *address},
		server.NewValidateService(myValidator{}),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	<-ctx.Done()
	if err := validator.Stop(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

The response and the `error` mean different things, and the split is the whole point
of the contract: the response carries what is wrong with the configuration — the check
working correctly — while an `error` means the check could not be made. Panics are recovered
and reported as `Internal`, so a bug surfaces instead of looking like a process that
died mid-call; the stack stays in the validator's log.

## Calling a validator in Go

Import [`api/validate/v1`](api/validate/v1) and [`client`](client). The caller owns
the process, the address and the deadlines:

```go
conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
// …
defer conn.Close()

// NewConfig() carries the protocol's message-size limits; appending keeps them.
config := client.NewConfig()
config.GRPCOptions = append(config.GRPCOptions, grpc.WaitForReady(true))

resp, err := client.NewValidateClient(conn, config).Validate(ctx, input)
if err != nil {
	return err // the validator failed
}
if len(resp.GetErrors()) > 0 {
	return errors.New(violationsText(resp.GetErrors())) // the configuration is wrong
}
return nil
```

The text is the caller's to render — the protocol carries the fields, not the wording:

```go
func violationsText(violations []*validatev1.ViolationResponse) string {
	lines := make([]string, 0, len(violations))

	for _, violation := range violations {
		if violation.GetPath() == "" {
			lines = append(lines, violation.GetMessage())

			continue
		}

		lines = append(lines, violation.GetPath()+": "+violation.GetMessage())
	}

	return strings.Join(lines, "\n")
}
```

## Adding an action

Each action is its own service under [`api/pb`](api/pb), its own package under
[`api`](api) and its own `server.Service` constructor, so a binary that does not
implement one simply does not pass it to `server.Start` and a caller sees
`Unimplemented`. The full recipe is in the module documentation ([`doc.go`](doc.go)).
