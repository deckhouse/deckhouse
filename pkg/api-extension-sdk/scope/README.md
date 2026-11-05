# scope

The receiver side of scoped reads of aggregated APIs in Deckhouse.

With `--scope`, kube-apiserver lets a user list, get and watch resources in the namespaces they may
read, even without the right to read them cluster-wide. For an aggregated API that opted in with the
`scope.deckhouse.io/resources` annotation on its `APIService`, kube-apiserver computes that set of
namespaces and sends it to the extension server in the `X-Deckhouse-Scope-Mode` and
`X-Deckhouse-Scope-Namespaces` headers. This package checks that the headers came from
kube-apiserver, lets the read through the delegated authorizer, and restricts what the storage
serves to the set.

The contract and the design are described in
[SCOPE_AGGREGATED_API.md](../../../modules/140-user-authz/docs/internal/SCOPE_AGGREGATED_API.md).

## Installation

```bash
go get github.com/deckhouse/deckhouse/pkg/api-extension-sdk
```

## Usage

### `k8s.io/apiserver`-based server

Put `WithTrusted` outside the default handler chain (`WithAuthentication` deletes the headers it needs)
and wrap the delegated authorizer for the opted-in resources:

```go
var gizmos = schema.GroupResource{Group: "example.deckhouse.io", Resource: "gizmos"}

cfg.Authorization.Authorizer = scope.WithAuthorizer(cfg.Authorization.Authorizer, gizmos)
cfg.BuildHandlerChainFunc = func(h http.Handler, c *genericapiserver.Config) http.Handler {
	return scope.WithTrusted(genericapiserver.DefaultBuildHandlerChain(h, c), c.Authentication.RequestHeaderConfig)
}
```

Then restrict the data in the REST storage:

```go
func (s *storage) List(ctx context.Context, opts *metainternalversion.ListOptions) (runtime.Object, error) {
	list := s.list(ctx, opts)
	if err := scope.FilterList(ctx, list); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *storage) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	ns := genericapirequest.NamespaceValue(ctx)
	if err := scope.CheckGet(ctx, gizmos, ns, name); err != nil {
		return nil, err
	}
	return s.get(ctx, ns, name)
}

func (s *storage) Watch(ctx context.Context, opts *metainternalversion.ListOptions) (watch.Interface, error) {
	w, err := s.watch(ctx, opts)
	if err != nil {
		return nil, err
	}
	return scope.WatchFilter(ctx, w), nil
}
```

Filter in the storage, never on the encoded response: the handler converts the result to a Table after
the storage returns, and a watch is encoded per event.

### Plain `net/http` server

The server must terminate TLS itself and request client certificates
(`tls.Config.ClientAuth: tls.RequestClientCert` or stricter). `ConfigMapTrustSource` follows the
requestheader CA and allowed names in `kube-system/extension-apiserver-authentication`:

```go
src := scope.NewConfigMapTrustSource(client, 10*time.Minute)
go src.Run(ctx)
handler := scope.NewTrustedHandler(mux, src)
```

The handler only decides whether the scope headers are honoured; authentication and authorization of
the caller stay with the server. Use `Allowed`, `Restricted`, `Filter` or `FromContext` to restrict
the data.

## API

| Layer | Functions |
|---|---|
| Trust | `WithTrusted`, `NewTrustedHandler`, `ConfigMapTrustSource`, `StaticTrustSource` |
| Authorization | `WithAuthorizer` |
| Filtering | `Allowed`, `Restricted`, `Filter`, `FilterList`, `WatchFilter`, `CheckGet` |
| Decision | `FromContext`, `WithDecision` (for tests of storage code) |

`FromContext` is the only source of the decision: the headers are deleted from every request after
the trusted handler, so nothing further down can read them.

## Example

[scope-aggregated-api-e2e](../../../modules/140-user-authz/docs/internal/scope-aggregated-api-e2e/README.md)
is a toy aggregated API server built on this package, with manifests and an end-to-end test.
