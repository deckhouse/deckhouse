---
title: "The kube-dns module: examples"
---

## Configuration example

```yaml
kubeDns: |
  upstreamNameservers:
  - 8.8.8.8
  - 8.8.4.4
  hosts:
  - domain: one.example.com
    ip: 192.168.0.1
  - domain: two.another.example.com
    ip: 10.10.0.128
  stubZones:
  - zone: consul.local
    upstreamNameservers:
    - 10.150.0.1
  enableLogs: true
  clusterDomainAliases:
  - foo.bar
  - baz.qux
```
