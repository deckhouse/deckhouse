---
title: "The l2-load-balancer module: примеры"
---

## Публикация сервиса через L2LoadBalancer

{% raw %}
* Настройте пул адресов:

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

* Подготовьте приложение, которое хотите опубликовать:

  ```bash
  kubectl create deploy nginx --image=nginx
  ```

* Установите ресурс _L2LoadBalancer_:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: L2LoadBalancer
  metadata:
    name: nginx
  spec:
    addressPool: mypool
    nodeSelector:
      node-role.kubernetes.io/loadbalancer: "" # селектор узлов-балансировщиков
    service:
      ports:
      - name: http
        port: 8000
        protocol: TCP
        targetPort: 80
      selector:
        app: nginx
  ```

  В результате, будут созданы сервисы с типом LoadBalancer:

  ```bash
  $ kubectl get svc
  NAME                          TYPE           CLUSTER-IP      EXTERNAL-IP       PORT(S)          AGE
  d8-l2-load-balancer-nginx-0   LoadBalancer   10.222.24.22    192.168.199.103   8000:31262/TCP   1s
  d8-l2-load-balancer-nginx-1   LoadBalancer   10.222.91.98    192.168.199.104   8000:30806/TCP   1s
  d8-l2-load-balancer-nginx-2   LoadBalancer   10.222.186.57   192.168.199.105   8000:30272/TCP   1s
  ```
  
  Назначенные IP адреса можно также увидеть в разделе Status объекта L2LoadBalacner:
  
* ```bash
  kubectl describe l2loadbalancers.deckhouse.io nginx
  ...
  Status:
    Public Addresses:
      192.168.199.103
      192.168.199.104
      192.168.199.105
  ```

  Полученные EXTERNAL-IP можно прописывать в качестве A-записей для прикладного домена.

  ```bash
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.103:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.104:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.105:8000
  200
  ```

{% endraw %}
