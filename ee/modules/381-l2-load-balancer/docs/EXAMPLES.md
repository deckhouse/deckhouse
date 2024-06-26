---
title: "The l2-load-balancer module: examples"
---

## Publishing an application via L2LoadBalancer

{% raw %}
* Configure the module:

  ```
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: l2-load-balancer
  spec:
    enabled: true
    settings:
      loadBalancerClass: l2-class
    version: 1
  ```

* Prepare the application to publish:

  ```
  kubectl create deploy nginx --image=nginx
  ```

* Deploy the _L2LoadBalancer_ resource:

  ```
  apiVersion: network.deckhouse.io/v1alpha1
  kind: L2LoadBalancer
  metadata:
    name: ingress
  spec:
    addressPool:
    - 192.168.2.100-192.168.2.150
    nodeSelector:
      node-role.kubernetes.io/loadbalancer: "" 
  ```

* Deploy standard resource _Service_ with special annotations:

  ```
  apiVersion: v1
  kind: Service
  metadata:
  name: nginx-deployment
  annotations:
    network.deckhouse.io/l2-load-balancer-name: ingress
    network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
  spec:
    ports:
    - port: 8000
      protocol: TCP
      targetPort: 80
    selector:
      app: nginx
    type: LoadBalancer
    loadBalancerClass: l2-class
  ```

  As a result, the created Service with the type `LoadBalancer` will be assigned the specified number of addresses.:

  ```
  $ kubectl get svc
  NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)        AGE
  nginx-deployment       LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30544/TCP   11s
  ```

  The resulting EXTERNAL-IP are ready to use in application DNS-domain.

  ```
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.2.100:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.2.101:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.2.102:8000
  200
  ```

{% endraw %}
