# Patches


### 001-endpointslices
EndpointSlices support for ServiceMonitor in the prometheus-operator is disabled by default. 
We enable it by checking EndpointSlice API in a Kubernetes cluster. It's enabled from version 1.21 so it should work always.
Also add Alertmanager support via EndpointSlice.
Upstream has 2 issues, why it's not enabled by default:
- https://github.com/prometheus-operator/prometheus-operator/pull/5291
- https://github.com/prometheus-operator/prometheus-operator/issues/3862#issuecomment-1068260430


### 002-endpointslices_fallback
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

### 003_alertmanager_tls_assets
Prometheus operator does not save TLS assets for alertmanager Webhook and Email recievers in the secret which mounted in alert manager pod. This patch fix it.
