# dhctl External Provider Protocol

This document describes the protocol between dhctl and external provider binaries.

## Overview

An external provider binary is a standalone executable placed in the plugins directory.
dhctl discovers it by name and invokes it as a subprocess for validation and resource
preparation steps during bootstrap, converge, and destroy operations.

## Binary location

The binary must be named `validator` and placed in a provider-named subdirectory
inside the plugins directory.

The terraform-manager image is unpacked into the download root directory.
The validator binary must be placed at `/<provider-name>/validator` inside the image,
which maps to `<download-root>/<provider-name>/validator` on disk.

```
<download-root>/
  dvp/
    validator        ← this binary
```

Examples:
- `dvp/validator`
- `aws/validator`

dhctl discovers the binary by looking for `<download-root>/<provider-name>/validator`.

## Subcommands

The binary is invoked with a single subcommand argument:

```
validator validate
validator prepare
```

## Transport

- Input: JSON object written to **stdin**, followed by a newline.
- Output: JSON object written to **stdout**, followed by a newline.
- Errors: diagnostic messages may be written to **stderr** (ignored by dhctl).

## Protocol version

Every request includes a `version` field with the protocol version string.
The current version is `"1"`.

A binary **must** reject requests with an unknown version by returning an error
response (for `validate`/`prepare`) or by exiting non-zero if the request cannot
be decoded at all.

## Subcommand: validate

Validates provider credentials and configuration before the operation begins.

**stdin:**
```json
{
  "version": "1",
  "input": {
    "providerName": "dvp",
    "operation": "bootstrap",
    "clusterPrefix": "my-cluster",
    "layout": "Standard",
    "providerClusterConfiguration": { ... },
    "resourcesYAML": "---\napiVersion: ...",
    "moduleConfig": { ... }
  }
}
```

**stdout:**
```json
{}
```

On validation failure:
```json
{"error": "human-readable error message"}
```

**Exit code:** always `0`. Non-zero exit means the binary itself crashed.

## Subcommand: prepare

Parses provider resources and returns structured variables for Terraform/OpenTofu.

**stdin:** same structure as validate.

**stdout:**
```json
{
  "result": {
    "vars": {
      "settings": { ... },
      "nodeGroups": { "worker": { ... } },
      "instanceClasses": { "worker": { ... } },
      "secrets": { "credentials": { ... } }
    },
    "providerClusterConfiguration": { ... }
  }
}
```

On failure:
```json
{"error": "human-readable error message"}
```

**Exit code:** always `0`.

## Input fields

| Field | Type | Description |
|---|---|---|
| `providerName` | string | Provider identifier, e.g. `"dvp"` |
| `operation` | string | One of `"bootstrap"`, `"converge"`, `"destroy"` |
| `clusterPrefix` | string | Prefix for cloud resource names |
| `layout` | string | Provider layout name |
| `providerClusterConfiguration` | object | Parsed `providerClusterConfiguration` section |
| `resourcesYAML` | string | Raw multi-document YAML with provider resources |
| `moduleConfig` | object | Cloud-provider module configuration values |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success (including business errors encoded in the response JSON) |
| non-zero | Binary crashed or could not parse stdin |

## Implementing a binary in Go

Use the `Handler` type from this package:

```go
import (
    "context"
    "fmt"
    "os"

    proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

func main() {
    h := proto.Handler{
        Validate: myValidate,
        Prepare:  myPrepare,
    }
    if err := h.Run(context.Background()); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

func myValidate(ctx context.Context, input proto.PrepareInput) error {
    // Return non-nil to signal validation failure.
    return nil
}

func myPrepare(ctx context.Context, input proto.PrepareInput) (*proto.PrepareResult, error) {
    return &proto.PrepareResult{
        Vars: &proto.CloudProviderVars{
            // fill from input.ResourcesYAML / input.ModuleConfig
        },
    }, nil
}
```
