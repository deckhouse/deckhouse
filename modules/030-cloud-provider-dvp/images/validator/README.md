# DVP validator

External provider binary for dhctl. Implements the `validate` and `prepare` subcommands
of the [dhctl external provider protocol](../../../../../go_lib/dhctl-provider-protocol).

## How it works

dhctl invokes this binary as a subprocess with one argument — `validate` or `prepare`.
The request JSON is read from stdin; the response JSON is written to stdout.
Diagnostics go to stderr.

### validate

Runs validation in `images/validator/src` using shared libraries:

- generic checks from `go_lib/cloud-provider/validation`;
- DVP-specific rules from `modules/030-cloud-provider-dvp/pkg/validation`;
- cluster state assembly via `go_lib/cloud-provider/validation/protocol.StateBuilder`
  configured with constants from `modules/030-cloud-provider-dvp/pkg/validation`.

The validator checks:

1. **ModuleConfig** — resource is present and named `cloud-provider-dvp`.
2. **CredentialSecret** — `d8-credentials` must have
   `type: cloud-provider.deckhouse.io/credentials`, `authScheme: kubeconfig`, and a
   valid kubeconfig in `secret` (stored as base64).
3. **etcdDisk attachment** — `DVPInstanceClass.spec.etcdDisk` is allowed only for the
   class attached to `NodeGroup/master`.
4. **Preflight (`bootstrap` + `converge`)** — requires:
   `Secret/d8-credentials`, `NodeGroup/master` with `DVPInstanceClass` reference,
   and `spec.etcdDisk` in that referenced class.

Migration from ProviderClusterConfiguration is skipped while legacy resources are
incomplete (`MigrationStatus` / `d8-module-is-migrating` ConfigMap).

**Preflight policy:** Preflight runs only in the dhctl validator for `bootstrap` and
`converge`. The admission webhook validates resource invariants only.

| Path                     | ModuleConfig | Credentials semantics | etcdDisk attachment | Preflight | Migration skip |
| ------------------------ | ------------ | --------------------- | ------------------- | --------- | -------------- |
| dhctl bootstrap/converge | yes          | yes (if present)      | yes                 | yes       | PCC / migration status |
| dhctl other operations   | yes          | yes (if present)      | yes                 | no        | PCC / migration status |
| admission webhook        | yes          | yes (if present)      | yes                 | no        | ConfigMap      |

On success writes `{}` to stdout and exits 0.
On validation error writes `{"error":"..."}` to stdout and exits 0.
On protocol/decode error writes to stderr and exits 1.

### prepare

Parses `resourcesYAML` into structured `vars` (NodeGroups, InstanceClasses, Secrets)
and returns them together with the unchanged `providerClusterConfiguration`.

`vars` population rules (from `go_lib/dhctl-provider-protocol/parse.go`):

| Field             | Condition                                                                        |
| ----------------- | -------------------------------------------------------------------------------- |
| `nodeGroups`      | `kind: NodeGroup`, `apiVersion: deckhouse.io/*`, `spec.nodeType: CloudPermanent` |
| `instanceClasses` | `kind` ends with `InstanceClass`, `apiVersion: deckhouse.io/*`                   |
| `secrets`         | `kind: Secret`, `type: cloud-provider.deckhouse.io/credentials`                  |

## Build

```bash
cd src
go build -o /tmp/dvp-validator .
```

## Manual testing

### validate — preflight requirements for converge

```bash
cat > /tmp/req.json << 'EOF'
{
  "version": "1",
  "input": {
    "providerName": "dvp",
    "clusterPrefix": "test",
    "layout": "standard",
    "operation": "converge",
    "providerClusterConfiguration": {},
    "moduleConfig": {
      "provider": {"parameters": {"namespace": "default"}},
      "storage": {"enabled": true, "parameters": {}},
      "nodes": {"enabled": false}
    },
    "resourcesYAML": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: d8-credentials\n  namespace: d8-cloud-provider-dvp\ntype: cloud-provider.deckhouse.io/credentials\nstringData:\n  authScheme: kubeconfig\n  secret: YXBpVmV=\n"
  }
}
EOF
/tmp/dvp-validator validate < /tmp/req.json
# {"error":"NodeGroup/master: NodeGroup \"master\" is required"}
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
