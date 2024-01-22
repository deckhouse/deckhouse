---
title: "The node-local-dns module: examples"
---

## An example of configuring a custom DNS for a Pod

The example is a definition of a Pod named `dns-example`, which does not use DNS services and runs a container with the image `nginx`.

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
