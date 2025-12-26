Sample EnvoyFilter for Deckhouse `geoproxy` (gRPC ext_proc)

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: EnvoyFilter
metadata:
  name: geoproxy-ext-proc
  namespace: istio-system
spec:
  workloadSelector:
    labels:
      istio: ingressgateway
  configPatches:
  - applyTo: HTTP_FILTER
    match:
      context: GATEWAY
      listener:
        filterChain:
          filter:
            name: envoy.filters.network.http_connection_manager
            subFilter:
              name: envoy.filters.http.router
    patch:
      operation: INSERT_BEFORE
      value:
        name: envoy.filters.http.ext_proc
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor
          grpc_service:
            envoy_grpc:
              # geoproxy Service: d8-ingress-nginx/geoproxy port 50051 (grpc).
              cluster_name: outbound|50051||geoproxy.d8-ingress-nginx.svc.cluster.local
          processing_mode:
            request_header_mode: SEND
            response_header_mode: SKIP
            request_body_mode: NONE
            response_body_mode: NONE
          failure_mode_allow: true
```

If your mesh uses STRICT mTLS and has no sidecar, add a DestinationRule to disable TLS for it:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: geoproxy-disable-mtls
  namespace: istio-system
spec:
  host: geoproxy.d8-ingress-nginx.svc.cluster.local
  trafficPolicy:
    tls:
      mode: DISABLE
```
