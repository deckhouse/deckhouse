---
title: "The log-shipper module: FAQ"
---

## How to add authorization to the _ClusterLogDestination_ resource?

To add authorization parameters to the [ClusterLogDestination](cr.html#clusterlogdestination) resource, you need to:
- change the [connection protocol](cr.html#clusterlogdestination-v1alpha1-spec-loki-endpoint) to Loki to HTTPS
- add the [auth](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth) section, in which:
  - the [strategy](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-strategy) parameter should be set to `Bearer`;
  - the [token](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-token) parameter should contain the `log-shipper-token` token from the `d8-log-shipper` namespace.

For example:

- ClusterLogDestination resource without authorization:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: loki
  spec:
    type: Loki
    loki:
      endpoint: "http://loki.d8-monitoring:3100"
  ```

- Get the `log-shipper-token` token from the `d8-log-shipper` namespace:

  ```bash
  kubectl -n d8-log-shipper get secret log-shipper-token -o jsonpath='{.data.token}' | base64 -d
  ```

- ClusterLogDestination resource with authorization:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: loki
  spec:
    type: Loki
    loki:
      endpoint: "https://loki.d8-monitoring:3100"
      auth:
        strategy: "Bearer"
        token: <log-shipper-token>
      tls:
        verifyHostname: false
        verifyCertificate: false
  ```
