---
title: "The metallb module: examples"
---

{% raw %}

Metallb can be used in Static (Bare Metal) clusters when you can't order a load balancer from a cloud provider. Metallb can work in L2 or BGP modes. Below is an example of Metallb usage in L2 mode.

We will create an Ingress Controller with `LoadBalancer` inlet. And we will also expose standalone Nginx web server using a Service with the `LoadBalancer` type.

First, you have to decide, which NodeGroups will be used to deploy applications that have to be exposed by the LoadBalancer service.
Ingress controllers run on frontend nodes, and Nginx web server runs on a worker node in this example. They have common label `node-role/metallb=""`.

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: frontend
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    labels:
      node-role.deckhouse.io/frontend: ""
      node-role/metallb: ""
    taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  disruptions:
    approvalMode: Manual
  nodeTemplate:
    labels:
      node-role/metallb: ""
  nodeType: Static
```

Check that nodes have the correct labels.

```bash
kubectl get nodes -l node-role/metallb
NAME              STATUS   ROLES      AGE   VERSION
demo-frontend-0   Ready    frontend   61d   v1.21.14
demo-frontend-1   Ready    frontend   61d   v1.21.14
demo-worker-0     Ready    worker     61d   v1.21.14
```

Module `metallb` is disabled by default, so you have to explicitly enable it. You also have to set the correct `nodeSelector` and `tolerations` for Metallb speakers.

An example of the module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  version: 1
  enabled: true
  settings:
    addressPools:
    - addresses:
      - 192.168.199.100-192.168.199.102
      name: frontend-pool
      protocol: layer2
    speaker:
      nodeSelector:
        node-role/metallb: ""
      tolerations:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        operator: Equal
        value: frontend
```

Create `IngressNginxController`.

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

Check that service with the type `LoadBalancer` is created in the namespace `d8-ingress-nginx`.

```shell
kubectl -n d8-ingress-nginx get svc main-load-balancer 
NAME                 TYPE           CLUSTER-IP       EXTERNAL-IP       PORT(S)                      AGE
main-load-balancer   LoadBalancer   10.222.255.194   192.168.199.100   80:30236/TCP,443:32292/TCP   30s
```

Your Ingress controller is accessible on an external IP address.

```shell
curl -s -o /dev/null -w "%{http_code}" 192.168.199.100
404
```

Expose your standalone Nginx web server on `8080` port.

```shell
kubectl create deploy nginx --image=nginx
kubectl create svc loadbalancer nginx --tcp=8080:80
```

Check service.

```shell
kubectl get svc nginx
NAME    TYPE           CLUSTER-IP     EXTERNAL-IP       PORT(S)          AGE
nginx   LoadBalancer   10.222.9.190   192.168.199.101   8080:31689/TCP   3m11s
```

Now you can access the application using curl.

```shell
curl -s -o /dev/null -w "%{http_code}" 192.168.199.101:8080
200
```

{% endraw %}
