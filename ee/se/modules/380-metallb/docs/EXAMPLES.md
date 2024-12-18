---
title: "The MetalLB module: examples"
---

Metallb can be used in Static (Bare Metal) clusters when there is no option to use cloud load balancers. Metallb can work in L2 LoadBalancer or BGP modes LoadBalancer.

## Example of MetalLB usage in L2 LoadBalancer mode

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
kubectl create deploy nginx --image=nginx
```

Deploy the _MetalLoadBalancerClass_ resource:

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

Deploy standard resource _Service_ with special annotation and MetalLoadBalancerClass name:

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
$ kubectl get svc
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

## Example of MetalLB usage in BGP LoadBalancer mode

{% raw %}

Enable the module and configure all the necessary parameters:

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
      tolerations:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        operator: Equal
  version: 2
```

Configure BGP peering on the network equipment.

{% endraw %}

## Additional configuration examples for _Service_

{% raw %}

To create a Services with shared IP addresses, you need to add the annotation `metallb.universe.tf/allow-shared-ip` to them:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns-service-tcp
  namespace: default
  annotations:
    metallb.universe.tf/allow-shared-ip: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  loadBalancerIP: 1.2.3.4
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
    metallb.universe.tf/allow-shared-ip: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  loadBalancerIP: 1.2.3.4
  ports:
    - name: dnsudp
      protocol: UDP
      port: 53
      targetPort: 53
  selector:
    app: dns
```

To create a _Service_ with a forcibly selected address in L2 LoadBalancer mode, you need to add the annotation `network.deckhouse.io/load-balancer-ips`:

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

To create a _Service_ with a forcibly selected address in BGP LoadBalancer mode, you need to add the annotation `metallb.universe.tf/loadBalancerIPs`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.universe.tf/loadBalancerIPs: 192.168.1.100
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

Creating a _Service_ and assigning it _IPAddressPools_ is possible in BGP LoadBalancer mode using the annotation `metallb.universe.tf/address-pool`. For L2 LoadBalancer mode, you need to use the _MetalLoadBalancerClass_ settings (see above).

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
