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

1. **ModuleConfig** (invariants) — resource is present and named `cloud-provider-dvp`.
2. **CredentialSecret content** (invariants) — if a managed credential Secret is present,
   it must have `type: cloud-provider.deckhouse.io/credentials`, `authScheme: kubeconfig`,
   and a valid base64 kubeconfig in `secret`.
3. **etcdDisk attachment** (invariants) — `DVPInstanceClass.spec.etcdDisk` is allowed only
   for the class attached to `NodeGroup/master`.
4. **Preflight (`bootstrap` + `converge` only)** — requires:
   - `Secret/d8-credentials` with credential type;
   - `NodeGroup/master` with `DVPInstanceClass` reference and `spec.etcdDisk` in that class;
   - when legacy `providerClusterConfiguration` contains `provider.kubeconfigDataBase64`,
     a non-empty valid base64 kubeconfig (replaces dhctl `dvp-kubeconfig` preflight).

Migration from ProviderClusterConfiguration is skipped while legacy resources are
incomplete (`MigrationStatus` / `d8-module-is-migrating` ConfigMap).

**Preflight policy:** Preflight runs only in the dhctl validator for `bootstrap` and
`converge`. The admission webhook validates resource invariants only.

| Path                     | ModuleConfig | Credential content | Credential presence | etcdDisk | Preflight | Migration skip |
| ------------------------ | ------------ | ------------------ | ------------------- | -------- | --------- | -------------- |
| dhctl bootstrap/converge | yes          | yes (if present)   | yes                 | yes      | yes       | PCC / migration status |
| dhctl other operations   | yes          | yes (if present)   | no                  | yes      | no        | PCC / migration status |
| admission webhook        | yes          | yes (if present)   | no                  | yes      | no        | ConfigMap      |

On success writes `{}` to stdout and exits 0.
On validation error writes `{"error":"..."}` to stdout and exits 0.
On protocol/decode error writes to stderr and exits 1.

### prepare

Returns `input.vars` and unchanged `providerClusterConfiguration`.
dhctl builds `vars` before calling the provider binary.

`vars` fields:

| Field             | Source |
| ----------------- | ------ |
| `settings`        | ModuleConfig settings for `cloud-provider-dvp` |
| `nodeGroups`      | CloudPermanent NodeGroups |
| `instanceClasses` | Provider InstanceClasses |
| `secrets`         | Credential Secrets in `d8-cloud-provider-dvp` |

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
    "vars": {
      "settings": {
        "provider": {"parameters": {"namespace": "default"}},
        "storage": {"enabled": true, "parameters": {}},
        "nodes": {"enabled": false}
      },
      "secrets": {
        "d8-credentials": {
          "metadata": {"name": "d8-credentials", "namespace": "d8-cloud-provider-dvp"},
          "type": "cloud-provider.deckhouse.io/credentials",
          "stringData": {"authScheme": "kubeconfig", "secret": "YXBpVmV="}
        }
      }
    }
  }
}
EOF
/tmp/dvp-validator validate < /tmp/req.json
# {"error":"NodeGroup/master: NodeGroup \"master\" is required"}
```

### prepare — passthrough vars

```bash
cat > /tmp/req.json << 'EOF'
{
  "version": "1",
  "input": {
    "providerName": "dvp",
    "operation": "bootstrap",
    "providerClusterConfiguration": {"apiVersion": "deckhouse.io/v1", "kind": "DVPClusterConfiguration"},
    "vars": {
      "settings": {"provider": {"parameters": {"namespace": "default"}}},
      "nodeGroups": {
        "worker": {
          "metadata": {"name": "worker"},
          "spec": {"nodeType": "CloudPermanent"}
        }
      }
    }
  }
}
EOF
/tmp/dvp-validator prepare < /tmp/req.json
# {"result":{"vars":{...},"providerClusterConfiguration":{...}}}
```

Manual test configs: `~/flant/bootstrap-configs/dvp/tests/` (see `TEST-CASES.md`).
