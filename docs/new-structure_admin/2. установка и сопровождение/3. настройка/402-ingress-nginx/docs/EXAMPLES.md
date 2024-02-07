---
title: "The ingress-nginx module: examples"
---

{% raw %}

## An example for AWS (Network Load Balancer)

When creating a balancer, all zones available in the cluster will be used.

In each zone, the balancer receives a public IP. If there is an instance with an Ingress controller in the zone, an A-record with the balancer's IP address from this zone is automatically added to the balancer's domain name.

When there are no instances with an Ingress controller in the zone, then the IP is automatically removed from the DNS.

If there is only one instance with an Ingress controller in a zone, when the pod is restarted, the IP address of the balancer of this zone will be temporarily excluded from DNS.

```yaml
apiVersion: deckhouse.io/v1
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

## An example for GCP / Yandex Cloud / Azure

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

> **Caution!** In **GCP**, nodes must have an annotation enabling them to accept connections to external addresses for the NodePort type services.

## An example for OpenStack

```yaml
apiVersion: deckhouse.io/v1
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

## An example for Bare metal

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

## An example for Bare metal (Behind external load balancer, e.g. Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp, etc.)

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

## An example for Bare metal (MetalLB Load Balancer)

The `metallb` module is currently available only in the Enterprise Edition version.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

In the case of using MetalLB, its speaker Pods must be run on the same Nodes as the Ingress controller Pods.

The controller must receive real IP addresses of clients — therefore its Service is created with the parameter `externalTrafficPolicy: Local` (disabling cross–node SNAT), and to satisfy this parameter the MetalLB speaker announce this Service only from those Nodes where the target Pods are running.

So for the current example [metallb module configuration](../380-metallb/configuration.html) should be like this:

```yaml
metallb:
 speaker:
   nodeSelector:
     node-role.deckhouse.io/frontend: ""
   tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

{% endraw %}
