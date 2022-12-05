---
title: "The basic-auth module: examples"
---

## An example of the configuration

```yaml
basicAuthEnabled: "true"
basicAuth: |
  locations:
  - location: "/"
    whitelist:
      - 1.1.1.1
    users:
      username: "password"
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

## Usage

Just add to the Ingress resource an annotation similar to the one below:

```yaml
nginx.ingress.kubernetes.io/auth-url: "http://basic-auth.kube-basic-auth.svc.cluster.local/"
```
