---
title: "The log-shipper module: FAQ"
---

## How to add authorization params to the ClusterLogDestination resource which is used to send logs to the d8-loki?

You need to change the `ClusterLogDestination` endpoint scheme to `https` and add the `auth` section with the `strategy` field set to `Bearer` and the `token` field set to the `log-shipper-token` token from the `d8-log-shipper` namespace.

For example:

ClusterLogDestination resource without authorization:
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

Get the `log-shipper-token` token from the `d8-log-shipper` namespace:
```bash
kubectl -n d8-log-shipper get secret log-shipper-token -o jsonpath='{.data.token}' | base64 -d
```

ClusterLogDestination resource with authorization:
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
