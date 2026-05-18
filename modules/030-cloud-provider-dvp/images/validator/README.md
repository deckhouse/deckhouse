# DVP validator

External provider binary for dhctl. Implements the `validate` and `prepare` subcommands
of the [dhctl external provider protocol](../../../../../go_lib/dhctl-provider-protocol).

## How it works

dhctl invokes this binary as a subprocess with one argument — `validate` or `prepare`.
The request JSON is read from stdin; the response JSON is written to stdout.
Diagnostics go to stderr.

### validate

Checks that the provider configuration is usable before bootstrap:

1. **kubeconfigDataBase64** — decodes and parses the kubeconfig, connects to the cluster,
   runs a `SelfSubjectReview` to confirm the service account identity. Only runs on
   `operation: bootstrap` and only when `providerClusterConfig` is non-empty.
2. **credential Secret** — checks that `resourcesYAML` contains at least one `Secret`
   with `type: cloud-provider.deckhouse.io/credentials`.

On success writes `{}` to stdout and exits 0.
On validation error writes `{"error":"..."}` to stdout and exits 0.
On protocol/decode error writes to stderr and exits 1.

### prepare

Parses `resourcesYAML` into structured `vars` (NodeGroups, InstanceClasses, Secrets)
and returns them together with the unchanged `providerClusterConfiguration`.

`vars` population rules (from `go_lib/dhctl-provider-protocol/parse.go`):

| Field | Condition |
|---|---|
| `nodeGroups` | `kind: NodeGroup`, `apiVersion: deckhouse.io/*`, `spec.nodeType: CloudPermanent` |
| `instanceClasses` | `kind` ends with `InstanceClass`, `apiVersion: deckhouse.io/*` |
| `secrets` | `kind: Secret`, `type: cloud-provider.deckhouse.io/credentials` |

## Build

```bash
cd src
go build -o /tmp/dvp-validator .
```

## Manual testing

### validate — missing Secret (expected error)

```bash
cat > /tmp/req.json << 'EOF'
{
  "version": "1",
  "input": {
    "providerName": "dvp",
    "clusterPrefix": "test",
    "layout": "standard",
    "operation": "bootstrap",
    "providerClusterConfiguration": {},
    "resourcesYAML": ""
  }
}
EOF
/tmp/dvp-validator validate < /tmp/req.json
# {"error":"DVP cloud provider config validation error: no credential Secret found..."}
```

### validate — skip kubeconfig check on non-bootstrap operation

```bash
cat > /tmp/req.json << 'EOF'
{
  "version": "1",
  "input": {
    "providerName": "dvp",
    "clusterPrefix": "test",
    "layout": "standard",
    "operation": "converge",
    "providerClusterConfiguration": {"provider": {"kubeconfigDataBase64": "invalid"}},
    "resourcesYAML": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\ntype: cloud-provider.deckhouse.io/credentials\n"
  }
}
EOF
/tmp/dvp-validator validate < /tmp/req.json
# {}
```

### prepare — full vars

```bash
cat > /tmp/req.json << 'EOF'
{
  "version": "1",
  "input": {
    "providerName": "dvp",
    "clusterPrefix": "test",
    "layout": "standard",
    "operation": "bootstrap",
    "providerClusterConfiguration": {"apiVersion": "deckhouse.io/v1", "kind": "DVPClusterConfiguration"},
    "resourcesYAML": "apiVersion: deckhouse.io/v1alpha1\nkind: DVPInstanceClass\nmetadata:\n  name: worker\nspec:\n  cpu: 4\n---\napiVersion: deckhouse.io/v1\nkind: NodeGroup\nmetadata:\n  name: static-worker\nspec:\n  nodeType: CloudPermanent\n---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: dvp-creds\ntype: cloud-provider.deckhouse.io/credentials\n",
    "moduleConfig": {"setting": "value"}
  }
}
EOF
/tmp/dvp-validator prepare < /tmp/req.json
# {"result":{"vars":{"settings":{"setting":"value"},"nodeGroups":{"static-worker":{...}},"instanceClasses":{"worker":{...}},"secrets":{"dvp-creds":{...}}},"providerClusterConfiguration":{...}}}
```

Note: `NodeGroup` with `nodeType: CloudEphemeral` is **not** included in `vars.nodeGroups` —
only `CloudPermanent` static nodes are passed to Terraform.
