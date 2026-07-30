# Patches

## 001-endpointslices.patch

EndpointSlices support for ServiceMonitor in the prometheus-operator is disabled by default. 
We enable it by checking EndpointSlice API in a Kubernetes cluster. It's enabled from version 1.21 so it should work always.
Also add Alertmanager support via EndpointSlice.
Upstream has 2 issues, why it's not enabled by default:
- https://github.com/prometheus-operator/prometheus-operator/pull/5291
- https://github.com/prometheus-operator/prometheus-operator/issues/3862#issuecomment-1068260430

## 002-endpointslices_fallback.patch

Client ServiceMonitors could have labels based on `__meta_kubernetes_endpoints_` metric.
So, we add labels mapping from `__meta_kubernetes_endpointslice_XXX` to `__meta_kubernetes_endpoints_XXX` and fire an alert
for those ServiceMonitors

mappings:
```
__meta_kubernetes_endpoints_name   - __meta_kubernetes_endpointslice_name
__meta_kubernetes_endpoints_label_XXXX  - __meta_kubernetes_endpointslice_label_XXXX
__meta_kubernetes_endpoints_annotation_XXX - __meta_kubernetes_endpointslice_annotation_XXX
__meta_kubernetes_endpoints_annotationpresent_XXX - __meta_kubernetes_endpointslice_annotationpresent_XXX
__meta_kubernetes_endpoint_node_name - __meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname
__meta_kubernetes_endpoint_ready - __meta_kubernetes_endpointslice_endpoint_conditions_ready
__meta_kubernetes_endpoint_port_name - __meta_kubernetes_endpointslice_port_name
__meta_kubernetes_endpoint_port_protocol - __meta_kubernetes_endpointslice_port_protocol
__meta_kubernetes_endpoint_address_target_kind - __meta_kubernetes_endpointslice_address_target_kind
__meta_kubernetes_endpoint_address_target_name - __meta_kubernetes_endpointslice_address_target_name
```

## 003-alertmanager_tls_assets.patch

Prometheus operator does not save TLS assets for alertmanager Webhook and Email recievers in the secret which mounted in alert manager pod. This patch fix it.

## 004-fix_cve.patch

Fixes several CVEs.

``` sh
go mod edit -go 1.23
go get golang.org/x/net@v0.33.0
go get github.com/golang-jwt/jwt/v4@v4.5.1
go get google.golang.org/protobuf@v1.33.0
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity@v1.6.0
go mod tidy
git diff
```

Additionally bump `golang.org/x/crypto` to v0.53.0, `golang.org/x/net` to v0.56.0, `golang.org/x/sys`
to v0.46.0, `golang.org/x/text` to v0.39.0 (and their transitive `golang.org/x/sync`,
`golang.org/x/term`) and `go.mongodb.org/mongo-driver` to v1.17.7 to fix CVEs reported by Trivy
against the embedded Go dependencies of `/bin/operator` and `/bin/prometheus-config-reloader`:
- `golang.org/x/crypto`: CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597
- `golang.org/x/net`: CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600
- `golang.org/x/sys`: CVE-2026-39824
- `golang.org/x/text`: CVE-2026-56852
- `go.mongodb.org/mongo-driver`: CVE-2026-2303

``` sh
go get golang.org/x/crypto@v0.53.0 golang.org/x/net@v0.56.0 golang.org/x/sys@v0.46.0 golang.org/x/text@v0.39.0 go.mongodb.org/mongo-driver@v1.17.7
go mod tidy
git diff go.mod go.sum
```

