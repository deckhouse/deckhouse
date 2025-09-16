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

For this example [metallb module configuration](../metallb/configuration.html) should be like this:

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
