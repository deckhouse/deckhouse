---
title: "Модуль ingress-nginx: пример конфигурации"
---

{% raw %}
## Общий пример
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  controllerVersion: "0.33"
  hsts: true
  config:
    gzip-level: "4"
    worker-processes: "8"
  additionalHeaders:
    X-Different-Name: "true"
    Host: "$proxy_host"
  acceptRequestsFrom:
  - 1.2.3.4/24
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Auto
      cpu:
        max: 100m
      memory:
        max: 200Mi
```

## Пример для AWS (Network Load Balancer)
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

## Пример для GCP
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

## Пример для Openstack
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
```

{% endraw %}
