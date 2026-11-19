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

A client can also present a client certificate instead of a token: see [Client certificates](#client-certificates).

The module depends on `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go` and `github.com/deckhouse/deckhouse/pkg/metrics-storage`, which brings `github.com/prometheus/client_golang` and `github.com/deckhouse/deckhouse/pkg/log`.

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
- if the sidecar has `--client-ca-file`, move the mount of the `kube-rbac-proxy-ca.crt` ConfigMap to the application container and set `ClientCAFile` to the mounted `ca.crt`: the Ingress controller presents a certificate, not a token (see [Client certificates](#client-certificates));
- if the application listened on `127.0.0.1` only for the sidecar, it no longer needs a separate plain HTTP port for metrics.

The rights of Prometheus stay as they were: it still needs `get` on the resource from `ResourceAttributes`.

## Configuration

`Config` fields:

- `Rules`: a path and the resource checked for it. `Path` is an `http.ServeMux` pattern without a method or a host:
  `/metrics` matches only itself, `/metrics/` matches the subtree, and the longest pattern wins.
  A request that matches no rule and no excluded path gets `404`, as with kube-rbac-proxy.
  A path that `http.ServeMux` does not accept, or two paths it cannot choose between, such as `/a/{x}` and `/a/{y}`, make `New` return an error.
- `ExcludePaths`: `path.Match` patterns, such as `/healthz` or `/ready*`, served without any check.
  Use them for probes: kubelet does not send a token.
- `ClientCAFile`: a PEM bundle of the CAs that sign client certificates. Not set by default:
  authentication is by token only. See [Client certificates](#client-certificates).
- `TokenCacheTTL` (2m), `AllowCacheTTL` (5m), `DenyCacheTTL` (30s): how long results are reused.
  A role granted to Prometheus applies within `DenyCacheTTL`; a revoked role stops working within `AllowCacheTTL`.
- `StaleCacheTTL` (1h30m): while the API server cannot be reached, the last results are used for this long
  after they expire, so metrics are still collected. This is the `--stale-cache-interval` of the sidecars
  in Deckhouse modules. A negative value turns it off.
- `RetryBackoff`: retries of a failed review, about 4 seconds in total by default. An answer that a retry
  cannot change, such as `Forbidden` when the application may not create reviews, is not retried.
- `ReviewTimeout` (5s): the time for all checks of one request. When the API server hangs instead of refusing
  connections, the cached decision is served after this time, before the default 10 second scrape timeout.
- `CacheSize` (4096): the number of tokens, of certificate checks and of decisions kept in memory, each.
- `Logger`: a `*slog.Logger` or anything else with its `Debug`, `Info`, `Warn`, `Error` and `Handler` methods, such as `*log.Logger` of `github.com/deckhouse/deckhouse/pkg/log`. `slog.Default()` if not set.
- `MetricStorage`: the storage of `pkg/metrics-storage` for the metrics of the middleware: see [Metrics](#metrics). Not set by default: the metrics then go to a storage of the middleware that nothing serves.

The HTTP method sets the verb of the check: `GET` and `HEAD` are `get`, `POST` is `create`, `PUT` is `update`,
`PATCH` is `patch`, `DELETE` is `delete`, and any other method needs `*`.

## Metrics

The middleware has metrics of its own, apart from the metrics of the endpoint it protects. They are off by default. To turn them on, pass a storage of `pkg/metrics-storage` in `Config.MetricStorage`: `New` registers the metrics in it, and the handler of the storage serves them together with the metrics of the application:

```go
metricStorage := metricsstorage.NewMetricStorage()

auth, err := kuberbacauth.New(client, kuberbacauth.Config{
	Rules:         rules,
	MetricStorage: metricStorage,
})
// ...
mux := http.NewServeMux()
mux.Handle("/metrics", metricStorage.Handler())
```

Give every middleware of the application the same storage: a second storage on the same registry cannot register the metrics again, and `New` returns an error.

| Metric | Labels | What it counts |
|---|---|---|
| `kube_rbac_auth_requests_total` | `result`, `credential` | Requests the middleware checked. `result` is `allowed`, `unauthorized` (401), `forbidden` (403) or `error` (500: the API server could not check the request and nothing was cached). `credential` is `certificate` for a trusted client certificate, `token` when the bearer token was checked, and `none` otherwise. Excluded paths and paths without a rule are not counted. |
| `kube_rbac_auth_review_requests_total` | `review`, `result` | Requests to the API server: `review` is `TokenReview` or `SubjectAccessReview`, `result` is `success` or `error`. Every retry is a request, and an answer from the cache is not. |
| `kube_rbac_auth_stale_results_total` | `review` | Results used after their lifetime because the API server could not answer: see `StaleCacheTTL`. The requests are still served, so this is how an unavailable API server shows. |

## TLS

`ListenAndServeTLS` serves HTTPS until the context is done and then shuts down gracefully.

- Without `CertFile` and `KeyFile` it generates a self-signed certificate at start, as kube-rbac-proxy does.
  The ServiceMonitors and PodMonitors of Deckhouse modules scrape such endpoints with `insecureSkipVerify: true`.
- With `CertFile` and `KeyFile` it rereads the files when they change, so a renewed certificate from a Secret
  works without a restart. The files are checked for new content at most every 5 seconds. If the new pair cannot be loaded, the previous one stays in use and a warning is logged.
- It closes a connection that is idle for `IdleTimeout`, 2 minutes by default. kube-rbac-proxy never closes them. Behind a proxy that keeps connections to the application open, set `IdleTimeout` above the idle timeout of the proxy: otherwise the proxy can send a request into a connection that is being closed.

An application with its own server can use only `Middleware.Handler`.

## Client certificates

`Config.ClientCAFile` lets a client in by its certificate instead of a token, as `--client-ca-file` of kube-rbac-proxy does:

```go
auth, err := kuberbacauth.New(client, kuberbacauth.Config{
	Rules:        rules,
	ClientCAFile: "/etc/kube-rbac-auth/client-ca/ca.crt",
})
// ...
return kuberbacauth.ListenAndServeTLS(ctx, kuberbacauth.ServerConfig{
	Address:   ":8082",
	Handler:   auth.Handler(mux),
	ClientCAs: auth.ClientCAs,
})
```

`ListenAndServeTLS` always asks clients for a certificate during the handshake, as kube-rbac-proxy does, so `ClientCAFile` alone turns certificate authentication on. A client without a certificate is still served and can use a token.

`ServerConfig.ClientCAs` is optional. With it the server names the accepted CAs in the handshake, so a client with several certificates sends the right one and a browser offers only the matching ones. The list follows the reloaded file. kube-rbac-proxy does not name the CAs.

An application with its own server sets `ClientAuth: tls.RequestClientCert` in its `tls.Config`. It can name the CAs too: `GetConfigForClient` returns a copy of that `tls.Config` with `ClientCAs` set to `auth.ClientCAs()`. The copy must list `h2` and `http/1.1` in `NextProtos`, or the connection falls back to HTTP/1.1. If the server has HTTP/2 turned off, for example with `GODEBUG=http2server=0`, the copy must list only `http/1.1`: a client that gets `h2` loses the connection.

Which CA goes into the file depends on the client:

- A Deckhouse module published through Ingress. This is what the sidecars with `--client-ca-file` in Deckhouse modules are for. The Ingress has the `nginx.ingress.kubernetes.io/proxy-ssl-use-controller-certificate: "true"` annotation, and the ingress controller presents its own certificate, signed by the kube-rbac-proxy CA of the platform. A module renders this CA into its namespace with the `helm_lib_kube_rbac_proxy_ca_certificate` template: the `ca.crt` key of the `kube-rbac-proxy-ca.crt` ConfigMap. Mount the ConfigMap and set `ClientCAFile` to the mounted `ca.crt`. Grant access to the groups of the controllers, `ingress-nginx:auth` and `alb:auth`.
- A person with a kubeconfig from a control plane node, such as `/etc/kubernetes/admin.conf`: it has a client certificate and no token. The CA of such certificates is in the `client-ca-file` key of the `extension-apiserver-authentication` ConfigMap in `kube-system`. A pod cannot mount a ConfigMap of another namespace, so a hook of the module has to copy the CA into the module's namespace.

Behind a proxy that ends TLS the middleware sees the certificate of the proxy, not of the person behind it. The proxy is the client: access is granted to the proxy, and the application gets no identity of the person. This is how the modules behind Ingress work.

The file can hold several CAs and is reread when it changes: it is checked for new content at most every 5 seconds. If the new bundle cannot be loaded, the previous one stays in use and a warning is logged.

Text around the certificates, such as comments, commented out certificates or the output of `openssl`, is skipped. A damaged PEM block or a PEM block of another type, such as a private key, makes the whole bundle invalid: loading the rest would silently drop a CA. kube-rbac-proxy skips what it cannot read.

A request with a trusted certificate is authenticated as in the API server: the common name is the user name and the organizations are the groups, plus `system:authenticated`. No `TokenReview` is made, and the access is checked with a `SubjectAccessReview` as usual. The certificate must be valid now. If it lists extended key usages, client authentication must be among them.

A client can send up to 4 intermediate CAs with its certificate. The result of a certificate check is reused for 10 seconds, but not longer than the certificate is valid.

A trusted certificate wins over a token. A certificate that is not trusted is ignored, so its owner can still use a token. The exception is a certificate that Go cannot parse, for example with an RSA key longer than 8192 bits: the handshake fails before the token is read, as with kube-rbac-proxy.

## Differences from kube-rbac-proxy

- An error of the API server without a cached result gives `500`, not `401`, so it is not mistaken for a bad token.
- `HEAD` is checked as `get`; kube-rbac-proxy sends an empty verb for it.
- Token audiences are not supported.
- The `Authorization` header reaches the application unchanged.
- A client certificate can come with at most 4 intermediate CAs.
- A client CA file with a damaged PEM block or a PEM block of another type is not loaded at all: see [Client certificates](#client-certificates).
- The HTTPS server closes idle connections: see [TLS](#tls).

## Testing

```shell
go test -race ./...
```
