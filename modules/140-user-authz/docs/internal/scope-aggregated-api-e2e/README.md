# Scoped reads of aggregated APIs: toy extension and e2e test

A minimal aggregated API server (`scopetoy.deckhouse.io/v1`, namespaced resource `gizmos`, kept in
memory) and an end-to-end test of scoped reads against it. See
[SCOPE_AGGREGATED_API.md](../SCOPE_AGGREGATED_API.md) for the design.

| Path | Contents |
|---|---|
| `internal/toy/`, `cmd/toy-apiserver/` | The toy server built on the receiver library [`pkg/api-extension-sdk/scope`](../../../../../pkg/api-extension-sdk/scope/README.md), used through a `replace` directive. |
| `deploy/` | Manifests: namespaces, RBAC, Deployment variants, Service and `APIService`, test users, e2e runner. |
| `test/e2e.sh` | The e2e checks, run from a workstation against the cluster. |

Unit tests run locally: `go test -race ./...`.

## Requirements

- kube-apiserver with the `scope-aggregated-api` patch (`images/kubernetes/kubernetes/patches/`) and
  `permission-browser-apiserver` of the same Deckhouse version.
- `kube-system/extension-apiserver-authentication` with a non-empty `requestheader-allowed-names`:
  the library trusts nothing when that list is empty.
- `d8`, `curl`, `jq` on the workstation.

## Deploy

All commands run from this directory.

### In-cluster build (primary)

No registry is needed. The sources of this directory and `pkg/api-extension-sdk` go into a ConfigMap, an init container builds the binary with
`golang:1.26` (the nodes must be able to pull it), and the main container runs it.

```shell
d8 k apply -f deploy/00-namespaces.yaml -f deploy/10-rbac.yaml
tar --exclude=bin -czf /tmp/scope-toy-src.tgz -C ../../../../.. pkg/api-extension-sdk modules/140-user-authz/docs/internal/scope-aggregated-api-e2e
d8 k -n scope-toy create configmap scope-toy-src --from-file=src.tgz=/tmp/scope-toy-src.tgz
d8 k apply -f deploy/incluster/20-deployment-incluster.yaml
d8 k apply -f deploy/30-service-apiservice.yaml -f deploy/40-test-users.yaml
d8 k -n scope-toy rollout status deploy/scope-toy
```

After a source change, recreate the ConfigMap and run `d8 k -n scope-toy rollout restart deploy/scope-toy`.

### Image from a registry

```shell
IMAGE=<registry>/scope-toy:dev bash hack/build.sh image   # docker build + push
IMAGE=<registry>/scope-toy:dev envsubst < deploy/20-deployment.yaml | d8 k apply -f -
```

Apply the other manifests as above. For a private registry add `imagePullSecrets` to the Deployment.
`deploy/hostpath/20-deployment-hostpath.yaml` runs a binary built by `bash hack/build.sh` and copied to
`/opt/scope-toy/` on a node. Apply it with `NODE=<node> BASE_IMAGE=<any image the node has> envsubst`;
it needs the `security.deckhouse.io/pod-policy=privileged` label on the `scope-toy` namespace.

## Run the e2e test

The test uses a temporary service account that may impersonate the test users only, instead of an
admin identity: never run the denied-user
checks as `system:masters`, the delegated authorizer always allows that group. The script reads the
current kubeconfig context and supports client certificates and static tokens only (no exec or OIDC
plugins).

```shell
d8 k apply -f deploy/50-e2e-runner.yaml

umask 077   # the kubeconfig below holds a token
TOKEN=$(d8 k -n scope-toy create token scope-toy-e2e --duration=2h)
d8 k config view --raw --minify --flatten -o json | jq --arg token "$TOKEN" '{
    apiVersion: "v1", kind: "Config",
    clusters: [.clusters[0]],
    users: [{name: "scope-toy-e2e", user: {token: $token}}],
    contexts: [{name: "scope-toy-e2e", context: {cluster: .clusters[0].name, user: "scope-toy-e2e"}}],
    "current-context": "scope-toy-e2e"}' > /tmp/scope-toy-e2e.kubeconfig

KUBECONFIG=/tmp/scope-toy-e2e.kubeconfig bash test/e2e.sh
```

The kubeconfig keeps only the server and CA of the current cluster. The script is not executable, so
run it with `bash`. Environment: `PROJECT=<name>` adds the `project:<name>` case,
`REVOKE_DEADLINE` (seconds, default 15) bounds watch revocation, `SKIP_PORT_FORWARD=1` skips the
direct-to-Service spoofing checks.

## Notes from a real stand

- The test namespaces become multitenancy-manager projects. A RoleBinding there may reference only
  ClusterRoles granted to the project, so the namespace readers use a namespaced Role
  (`deploy/40-test-users.yaml`).
- `permission-browser-apiserver` follows the `scope.deckhouse.io/resources` annotation through a
  watch on `APIService` objects, so a change takes effect within seconds.

## Cleanup

Delete the runner first, then everything else:

```shell
d8 k delete -f deploy/50-e2e-runner.yaml
rm -f /tmp/scope-toy-e2e.kubeconfig /tmp/scope-toy-src.tgz
d8 k delete -f deploy/30-service-apiservice.yaml -f deploy/40-test-users.yaml
d8 k delete -f deploy/10-rbac.yaml -f deploy/00-namespaces.yaml
```
