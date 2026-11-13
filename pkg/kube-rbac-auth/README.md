# kube-rbac-auth

`kube-rbac-auth` protects HTTP endpoints of a Go application, usually `/metrics`, in the same way as the
`kube-rbac-proxy` sidecar, but inside the application. The pod loses one container, and the access rules,
the Service and the ServiceMonitor stay as they were.

For each request the middleware:

1. takes the bearer token from the `Authorization` header;
2. checks the token with a `TokenReview`;
3. checks with a `SubjectAccessReview` that the token's user may access the resource configured for the path,
   for example `deployments/prometheus-metrics`;
4. passes the request to the application or answers `401`, `403` or `404`.

The module depends only on `k8s.io/api`, `k8s.io/apimachinery` and `k8s.io/client-go`.

## Installation

```shell
go get github.com/deckhouse/deckhouse/pkg/kube-rbac-auth
```

## Replacing the sidecar

A typical sidecar configuration in a module's templates. The names, the port and the excluded path are examples:

```yaml
- name: kube-rbac-proxy
  args:
  - "--secure-listen-address=$(KUBE_RBAC_PROXY_LISTEN_ADDRESS):8082"
  - "--stale-cache-interval=1h30m"
  env:
  - name: KUBE_RBAC_PROXY_CONFIG
    value: |
      excludePaths:
      - /healthz
      upstreams:
      - upstream: http://127.0.0.1:8080/metrics
        path: /metrics
        authorization:
          resourceAttributes:
            namespace: d8-example
            apiGroup: apps
            apiVersion: v1
            resource: deployments
            subresource: prometheus-metrics
            name: example
```

The same in the application:

```go
import (
	kuberbacauth "github.com/deckhouse/deckhouse/pkg/kube-rbac-auth"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
)

func serveMetrics(ctx context.Context, client kubernetes.Interface) error {
	auth, err := kuberbacauth.New(client, kuberbacauth.Config{
		Rules: []kuberbacauth.Rule{{
			Path: "/metrics",
			ResourceAttributes: kuberbacauth.ResourceAttributes{
				Namespace:   "d8-example",
				APIGroup:    "apps",
				APIVersion:  "v1",
				Resource:    "deployments",
				Subresource: "prometheus-metrics",
				Name:        "example",
			},
		}},
		ExcludePaths: []string{"/healthz"},
	})
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	return kuberbacauth.ListenAndServeTLS(ctx, kuberbacauth.ServerConfig{
		Address: ":8082",
		Handler: auth.Handler(mux),
	})
}
```

Then in the templates:

- remove the sidecar container and its `KUBE_RBAC_PROXY_CONFIG`;
- give the port `8082` (`https-metrics`) to the application container; the Service and the ServiceMonitor do not change;
- keep the ClusterRoleBinding of the application's ServiceAccount to the `d8:rbac-proxy` ClusterRole: it allows `create`
  on `tokenreviews` and `subjectaccessreviews`, which the middleware needs just as the sidecar did;
- if the application listened on `127.0.0.1` only for the sidecar, it no longer needs a separate plain HTTP port for metrics.

The rights of Prometheus stay as they were: it still needs `get` on the resource from `ResourceAttributes`.

## Configuration

`Config` fields:

- `Rules`: a path and the resource checked for it. `Path` is an `http.ServeMux` pattern without a method or a host:
  `/metrics` matches only itself, `/metrics/` matches the subtree, and the longest pattern wins.
  A request that matches no rule and no excluded path gets `404`, as with kube-rbac-proxy.
- `ExcludePaths`: `path.Match` patterns, such as `/healthz` or `/ready*`, served without any check.
  Use them for probes: kubelet does not send a token.
- `TokenCacheTTL` (2m), `AllowCacheTTL` (5m), `DenyCacheTTL` (30s): how long results are reused.
  A role granted to Prometheus applies within `DenyCacheTTL`; a revoked role stops working within `AllowCacheTTL`.
- `StaleCacheTTL` (1h30m): while the API server cannot be reached, the last results are used for this long
  after they expire, so metrics are still collected. This is the `--stale-cache-interval` of the sidecars
  in Deckhouse modules. A negative value turns it off.
- `RetryBackoff`: retries of a failed review, about 4 seconds in total by default. An answer that a retry
  cannot change, such as `Forbidden` when the application may not create reviews, is not retried.
- `ReviewTimeout` (5s): the time for all checks of one request. When the API server hangs instead of refusing
  connections, the cached decision is served after this time, before the default 10 second scrape timeout.
- `CacheSize` (4096): the number of tokens and the number of decisions kept in memory.
- `Logger`: `*slog.Logger`, `slog.Default()` if not set.

The HTTP method sets the verb of the check: `GET` and `HEAD` are `get`, `POST` is `create`, `PUT` is `update`,
`PATCH` is `patch`, `DELETE` is `delete`, and any other method needs `*`.

## TLS

`ListenAndServeTLS` serves HTTPS until the context is done and then shuts down gracefully.

- Without `CertFile` and `KeyFile` it generates a self-signed certificate at start, as kube-rbac-proxy does.
  The ServiceMonitors and PodMonitors of Deckhouse modules scrape such endpoints with `insecureSkipVerify: true`.
- With `CertFile` and `KeyFile` it rereads the files when they change, so a renewed certificate from a Secret
  works without a restart. If the new pair cannot be loaded, the previous one stays in use.

An application with its own server can use only `Middleware.Handler`.

## Differences from kube-rbac-proxy

- An error of the API server without a cached result gives `500`, not `401`, so it is not mistaken for a bad token.
- `HEAD` is checked as `get`; kube-rbac-proxy sends an empty verb for it.
- Client certificates and token audiences are not supported; authentication is by bearer token only.
- The `Authorization` header reaches the application unchanged.

## Testing

```shell
go test -race ./...
```
