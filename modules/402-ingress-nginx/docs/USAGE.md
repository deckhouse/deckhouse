---
title: "The ingress-nginx module: usage"
---

{% raw %}
## A general example
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
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Auto
      cpu:
        max: 100m
      memory:
        max: 200Mi
```

## An example for AWS (Network Load Balancer)
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
## An example for AWS (Network Load Balancer), Ingress nodes are not present in all zones

In this particular case, you need to create an annotation with all the subnet IDs where Listeners must be created. The subnets must correspond to the zones where the Ingress nodes are located.
You can get the list of subnets for a specific installation using the following command: `kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller module values cloud-provider-aws -o json | jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'`.

**Caution!** Note that attaching an annotation to a running Service will not work; you will need to recreate it.

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
      service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-foo, subnet-bar"
```

## An example for GCP
```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

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

## An example for bare metal

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
{% endraw %}
