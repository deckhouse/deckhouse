# DVP validator

External provider binary for dhctl. Implements the `validate` action of the
[dhctl provider validator protocol](../../../../../go_lib/dhctl-provider-protocol).

## How it works

dhctl starts this binary as a subprocess and calls it over gRPC:

```
validator serve --address=<address> [--network=<network>]
```

The command tree is cobra: `validator` alone prints help, `validator serve --help`
lists the flags.

`--address` is `host:port`: dhctl binds a loopback address per run and passes it in,
so there is no default. `--network` defaults to `tcp`, which the protocol's `server`
package also accepts, and then `--address` is a socket path instead. The binary serves
until `SIGTERM`, then stops gracefully. Diagnostics go to stderr; no part of the protocol travels through
stdout.

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

**Validation policy:** The dhctl validator runs preflight and invariant checks only
for `bootstrap` and `converge`. Other operations, such as `destroy`, only decode
the incoming state and then return successfully.

| Path                     | ModuleConfig | Credential content | Credential presence | etcdDisk | Preflight | Migration skip |
| ------------------------ | ------------ | ------------------ | ------------------- | -------- | --------- | -------------- |
| dhctl bootstrap/converge | yes          | yes (if present)   | yes                 | yes      | yes       | PCC / migration status |
| dhctl other operations   | no           | no                 | no                  | no       | no        | no             |
| admission webhook        | yes          | yes (if present)   | no                  | yes      | no        | ConfigMap      |

A valid configuration is an empty response. Violations travel as `errors` (blocking)
and `warnings` (not blocking); dhctl renders the blocking ones as the text of its
error. A failure of the validator itself is never a violation — it is a gRPC status:
`InvalidArgument` for a request the action rejects (an unknown `operation`, for
instance), `Internal` for a check that could not be made or a panic.


## Build

```bash
cd src
go build -o /tmp/dvp-validator .
```

## Manual testing

Start the validator:

```bash
/tmp/dvp-validator serve --network=tcp --address=127.0.0.1:18443 &
```

Call it with `grpcurl` from `src` (that is where the relative `-import-path` points).
The payload is a JSON-encoded `Input` wrapped in `input_json`, so it needs base64 —
reflection is not served, hence `-proto`:

```bash
INPUT=$(cat << 'EOF' | base64
{
  "providerName": "dvp",
  "clusterPrefix": "test",
  "layout": "standard",
  "operation": "converge",
  "providerClusterConfiguration": {},
  "vars": {
    "settings": {
      "provider": {"parameters": {"namespace": "default"}},
      "storage": {"disabled": false, "parameters": {}},
      "nodes": {"disabled": true}
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
EOF
)

grpcurl -plaintext \
  -import-path ../../../../../go_lib/dhctl-provider-protocol/api/pb \
  -proto validate/v1/validate.proto \
  -d "{\"input_json\": \"$INPUT\"}" \
  127.0.0.1:18443 dhctl.provider.validate.v1.ValidateService/Validate
# {"errors":[{"path":"Secret/d8-credentials","code":"credential_secret_required", …},
#             {"path":"NodeGroup/master","code":"master_node_group_required", …}]}

kill %1
```

