# 001-use-existing-control-plane-endpoint.patch

This directory contains patches applied to the upstream `yandex-cloud/cluster-api-provider-yandex`
source during the `capyandex-controller-manager` image build.

Adds control plane endpoint management modes:
- `ManagedLoadBalancer` keeps the upstream behavior.
- `External` skips load balancer reconciliation and requires
  `spec.controlPlaneEndpoint` to be set by the integrating platform.
