---
title: "The istio module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  istioEnabled: "true"
```

## Parameters

* `tlsMode` — the mode for transparent encryption of inter-pod traffic ([Mutual TLS](https://istio.io/latest/docs/tasks/security/authentication/mtls-migration/)).
    * Possible values:
        * `"Off"` — outgoing requests are not encrypted; incoming unencrypted requests are accepted. (Hint: always use quotation marks).
        * `"MutualPermissive"` — outgoing requests are encrypted; incoming unencrypted requests are accepted. This mode is useful when migrating to mTLS.
        * `"Mutual"` — outgoing requests are encrypted; incoming unencrypted requests are rejected (pods accept only encrypted requests).
    * The regular HTTP probes do not support the `"Mutual"` mode since kubelet knows nothing about mTLS. However, the istio sidecar has a dedicated port for these probes. The sidecar-injector that injects sidecar containers into pods can also route your probes to a dedicated port.
    * Each service will get an individual key signed by the root CA for encryption and authentication. The CA is either generated when the module starts for the first time or is specified explicitly (refer to the `ca` section).
    * It is `"Off"` by default.
    * You can manage the mTLS mode locally using the [AuthorizationPolicy](istio-cr.html#authorizationpolicy) and [DestinationRule](istio-cr.html#destinationrule) resources.
* `ca` — is an explicitly specified root certificate. It signs individual service certificates if mTLS is enabled (see `tlsMode`):
    * Format — a string in the PEM format.
    * `cert` — the root or intermediate certificate.
    * `key` — the key to the above certificate.
    * `chain` — a certificate chain if `cert` is an intermediate certificate.
    * `root` — the root certificate if `cert` is an intermediate certificate.
* `sidecar`:
    * `includeOutboundIPRanges` — traffic to these IP ranges is forcibly routed through Istio.
        * Format — an array of subnets.
        * By default, it is a list of Pod CIDRs, Service CIDRs, and node addresses collected by the `cluster_ip_ranges` discovery hook in the cluster. Thus, non-local traffic is handled as usual while local traffic is controlled.
        * You can redefine this parameter locally using the `traffic.sidecar.istio.io/includeOutboundIPRanges` annotation.
    * `excludeOutboundIPRanges` — traffic to these IP ranges is guaranteed not to flow through Istio.
        * Format — an array of subnets.
        * It is set to `[]` by default. That is, there are no exceptions.
        * You can redefine this parameter locally using the `traffic.sidecar.istio.io/excludeOutboundIPRanges` annotation.
    * `excludeInboundPorts` — the range of inbound ports whose traffic is guaranteed not to flow through Istio.
        * Format — an array of ports.
        * It is set to `[]` by default. That is, there are no exceptions.
        * You can redefine this parameter locally using the `traffic.sidecar.istio.io/excludeInboundPorts` annotation.
    * `excludeOutboundPorts` — the range of outbound ports whose traffic is guaranteed not to flow through Istio.
        * Format — an array of ports.
        * It is set to `[]` by default. That is, there are no exceptions.
        * You can redefine this parameter locally using the `traffic.sidecar.istio.io/excludeOutboundPorts` annotation.
* `federation` — parameters for federating with other clusters:
  * `enabled` — designate this cluster as a federation member (see [Enabling federation](./#enabling-federation)).
  * `ingressGateway` — ingressgateway settings:
    * `inlet` — the method for exposing ingressgateway:
      * `LoadBalancer` — is a recommended method if you have a cloud-based cluster and it supports Load Balancing.
      * `NodePort` — for installations that do not have the LB.
      * By default, it is set to `LoadBalancer`.
    * `nodeSelector` — the selector for the ingressgateway DaemonSet.
      * Format — a regular dictionary.
    * `tolerations` — for the ingressgateway DaemonSet.
      * Format — a regular array.
    * `serviceAnnotations` — additional service annotations. They can be used, e.g., for configuring a local LB in the Yandex.Cloud (using the `yandex.cpi.flant.com/listener-subnet-id` annotation).
      * Format — a regular dictionary.
* `multicluster` — multicluster parameters:
  * `enabled` — designate this cluster as a multicluster member (see [Enabling multicluster](./#enabling-multicluster)).
* `nodeSelector` —  the same as the pods' `spec.nodeSelector` parameter in Kubernetes.
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any nodeSelector.
* `tolerations` — the same as the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any tolerations.
