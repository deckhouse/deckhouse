---
title: "The l2-load-balancer module: examples"
---

## Publishing an application via L2LoadBalancer

{% raw %}
* Configure the address pool:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: l2-load-balancer
  spec:
    enabled: true
    settings:
      addressPools:
      - addresses:
        - 192.168.199.100-192.168.199.110
        name: mypool
    version: 1
  ```

* Prepare the application to publish:

  ```bash
  kubectl create deploy nginx --image=nginx
  ```

* Deploy the _L2LoadBalancer_ resource:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: L2LoadBalancer
  metadata:
    name: nginx
  spec:
    addressPool: mypool
    nodeSelector:
      node-role.kubernetes.io/loadbalancer: "" # load-balancing nodes
    service:
      ports:
      - name: http
        port: 8000
        protocol: TCP
        targetPort: 80
      selector:
        app: nginx
  ```

  As result, the _Services_ with type `LoadBalancer` will be created:

  ```bash
  $ kubectl get svc
  NAME                          TYPE           CLUSTER-IP      EXTERNAL-IP       PORT(S)          AGE
  d8-l2-load-balancer-nginx-0   LoadBalancer   10.222.24.22    192.168.199.103   8000:31262/TCP   1s
  d8-l2-load-balancer-nginx-1   LoadBalancer   10.222.91.98    192.168.199.104   8000:30806/TCP   1s
  d8-l2-load-balancer-nginx-2   LoadBalancer   10.222.186.57   192.168.199.105   8000:30272/TCP   1s
  ```
  
  The assigned IP addresses can also be seen in the Status section of the custom resource L2LoadBalancer:
  
* ```bash
  kubectl describe l2loadbalancers.deckhouse.io nginx
  ...
  Status:
    Public Addresses:
      192.168.199.103
      192.168.199.104
      192.168.199.105
  ```
  
  The resulting EXTERNAL-IP are ready to use in application DNS-domain.

  ```bash
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.103:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.104:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.105:8000
  200
  ```

{% endraw %}
