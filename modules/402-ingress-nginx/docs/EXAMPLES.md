---
title: "The ingress-nginx module: examples"
---

{% raw %}

## An example for AWS (Network Load Balancer)

When creating a balancer, all zones available in the cluster are used.

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

{% endraw %}

{% alert level="warning" %}
In **GCP**, nodes must have an annotation enabling them to accept connections to external addresses for the NodePort type services.
{% endalert %}

{% raw %}

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

{% endraw %}

## An example for Bare metal (MetalLB BGP LoadBalancer)

{% alert level="warning" %}This feature is available in Enterprise Edition only.{% endalert %}

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

The controller must receive real IP addresses of clients — therefore its Service is created with the parameter `externalTrafficPolicy: Local` (disabling cross–node SNAT), and to accept this parameter the MetalLB speaker announce this Service only from those Nodes where the target Pods are running.

For this example [metallb module configuration](/modules/metallb/configuration.html) should be like this:

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

## An example for Bare metal (MetalLB L2 LoadBalancer)

{% alert level="warning" %}This feature is available in the following editions: SE, SE+, EE.{% endalert %}

1. Enable the `metallb` module:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Deploy the _MetalLoadBalancerClass_ resource:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # label on load-balancing nodes
     type: L2
   ```

1. Deploy the _IngressNginxController_ resource:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: IngressNginxController
   metadata:
     name: main
   spec:
     ingressClass: nginx
     inlet: LoadBalancer
     loadBalancer:
       loadBalancerClass: ingress
       annotations:
       # The number of addresses that will be allocated from the pool declared in MetalLoadBalancerClass.
       network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
     # Label selector and tolerations. Ingress-controller pods must be scheduled on same nodes as MetalLB speaker pods.
     nodeSelector:
        node-role.kubernetes.io/loadbalancer: ""
     tolerations:
     - effect: NoSchedule
       key: node-role/loadbalancer
       operator: Exists
      ```

1. The platform will create a service with the type `LoadBalancer`, to which a specified number of addresses will be assigned:

   ```shell
   d8 k -n d8-ingress-nginx get svc
   NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)                      AGE
   main-load-balancer     LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30689/TCP,443:30668/TCP   11s
   ```

## Example of segregating access between public and administrative zones

In many applications, the same backend serves both the public part and the administrative interface. For example:

- `https://example.com` is the public zone;
- `https://admin.example.com` is the administrative zone, access to which must be restricted (`ACL`, `mTLS`, `IP whitelist`, and so on).

For this scenario, we recommend offloading administrative traffic to a separate Ingress controller (with a dedicated Ingress class if necessary) and restricting access to it by using the [`spec.acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter.

### Specifics of using a single Ingress controller

Consider an example where a single Ingress controller is used to serve requests from both the public zone and the administrative interface.

Example of Ingress resource configuration for this case:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: admin-ingress
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "1.2.3.4/32"
spec:
  ingressClassName: nginx # The Ingress resource for administrative traffic is associated with the same Ingress controller as the Ingress resource for public traffic.
  rules:
    - host: admin.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-ingress
spec:
  ingressClassName: nginx # The Ingress resource for public traffic is associated with the same Ingress controller as the Ingress resource for administrative traffic.
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
```

With [processing and forwarding of X-Forwarded-* headers enabled](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-hostport-behindl7proxy), the backend can rely on the `x-forwarded-host` header when making authorization decisions. In the example above, it is possible to reach the administrative zone through the Ingress resource that serves public traffic by using `x-forwarded-host`. Therefore, when using this option you must be sure that requests to the Ingress controller come only from trusted sources.

### Using separate Ingress controllers

To avoid the situation described above (when, with [processing and forwarding of X-Forwarded-* headers enabled](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-hostport-behindl7proxy) it is possible, for example, to reach the administrative zone via the Ingress resource that serves public traffic by using `x-forwarded-host`), we recommend that you:

- configure access rules at the Ingress resource level,
- use separate Ingress controllers,
- restrict which source addresses are allowed to connect to the Ingress controllers.

Example of Ingress resource configuration for this case:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: admin-ingress
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "1.2.3.4/32"
spec:
  ingressClassName: admin-nginx # The Ingress resource for administrative traffic is associated with a separate Ingress controller.
  rules:
    - host: admin.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-ingress
spec:
  ingressClassName: public-nginx # The Ingress resource for public traffic is associated with a separate Ingress controller.
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
```

Example of an Ingress controller that serves administrative Ingress resources and accepts connections only from specified subnets:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: admin
spec:
  ingressClass: admin-nginx
  inlet: HostPort
  acceptRequestsFrom:
    - 1.2.3.4/32
    - 10.0.0.0/16
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

In this example:

- The Ingress controller is exposed on node ports through the `HostPort` inlet.
- The [`acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter allows connections to the controller only from the listed subnets.
- Even if an external load balancer or client can set its own `X-Forwarded-*` header values, the decision whether to allow the connection to reach the controller is made based on the actual source address, not on headers.
- Administrative Ingress resources (in this example `admin-ingress`) are served by this controller according to the configured Ingress class.

Example of an Ingress controller that serves Ingress resources for public traffic:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: public
spec:
  ingressClass: public-nginx
  inlet: HostPort
  hostPort:
    httpPort: 8080
    httpsPort: 8443
    behindL7Proxy: true
```
