---
title: "The node-local-dns module: examples"
---

## An example of configuring a custom DNS for a Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dns-example
spec:
  dnsPolicy: "None"
  dnsConfig:
    nameservers:
      - 169.254.20.10
  containers:
    - name: test
      image: nginx
```

[Here](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config) you can learn more about DNS configuring.
