---
title: "The metallb module: examples"
---

Metallb can be used in Static (bare metal) clusters when there is no option to use cloud load balancers. Metallb can work in L2 LoadBalancer or BGP modes LoadBalancer.

## Example of metallb usage in L2 LoadBalancer mode

{% raw %}

Enable the module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 2
```

Prepare the application to publish:

```shell
d8 k create deploy nginx --image=nginx
```

Deploy the MetalLoadBalancerClass resource:

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
    node-role.kubernetes.io/loadbalancer: "" # node-balancer selector
  type: L2
```

Deploy standard resource Service with special annotation and MetalLoadBalancerClass name:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-deployment
  annotations:
    network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
spec:
  type: LoadBalancer
  loadBalancerClass: ingress # MetalLoadBalancerClass name
  ports:
  - port: 8000
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
```

As a result, the created Service with the type `LoadBalancer` will be assigned the specified number of addresses:

```shell
d8 k get svc
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)        AGE
nginx-deployment       LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30544/TCP   11s
```

The resulting EXTERNAL-IP are ready to use in application DNS-domain:

```shell
$ curl -s -o /dev/null -w "%{http_code}" 192.168.2.100:8000
200
$ curl -s -o /dev/null -w "%{http_code}" 192.168.2.101:8000
200
$ curl -s -o /dev/null -w "%{http_code}" 192.168.2.102:8000
200
```

{% endraw %}

## Example of metallb usage in BGP LoadBalancer mode

{% raw %}

Enable the module and configure all the necessary parameters<sup>*</sup>:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  settings:
    addressPools:
    - addresses:
      - 192.168.219.100-192.168.219.200
      name: mypool
      protocol: bgp
    bgpPeers:
    - hold-time: 3s
      my-asn: 64600
      peer-address: 172.18.18.10
      peer-asn: 64601
    speaker:
      nodeSelector:
        node-role.deckhouse.io/metallb: ""
  version: 2
```

<sup>*</sup> — in future versions, BGP mode settings will be set via the MetalLoadBalancerClass resource.

Configure BGP peering on the network equipment.

{% endraw %}

## Additional configuration examples for Service

{% raw %}

To create a Services with shared IP addresses, you need to add the annotation `network.deckhouse.io/load-balancer-shared-ip-key` to them:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns-service-tcp
  namespace: default
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  ports:
    - name: dnstcp
      protocol: TCP
      port: 53
      targetPort: 53
  selector:
    app: dns
---
apiVersion: v1
kind: Service
metadata:
  name: dns-service-udp
  namespace: default
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  ports:
    - name: dnsudp
      protocol: UDP
      port: 53
      targetPort: 53
  selector:
    app: dns
```

To create a Service with a forcibly selected address, you need to add the annotation `network.deckhouse.io/load-balancer-ips`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    network.deckhouse.io/load-balancer-ips: 192.168.217.217
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

Creating a Service and assigning it _IPAddressPools_ is possible in BGP LoadBalancer mode using the annotation `metallb.universe.tf/address-pool`. For L2 LoadBalancer mode, you need to use the MetalLoadBalancerClass settings (see above).

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.universe.tf/address-pool: production-public-ips
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

{% endraw %}
