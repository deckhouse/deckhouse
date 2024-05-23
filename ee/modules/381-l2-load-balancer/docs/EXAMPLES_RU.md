---
title: "The l2-load-balancer module: примеры"
---

## Публикация сервиса через L2LoadBalancer

{% raw %}
* Включите модуль:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: l2-load-balancer
  spec:
    enabled: true
    settings:
      loadBalancerClass: l2-class #опционально
    version: 1
  ```

* Подготовьте приложение, которое хотите опубликовать:

  ```shell
  kubectl create deploy nginx --image=nginx
  ```

* Создайте ресурс _L2LoadBalancer_:

  ```yaml
  apiVersion: network.deckhouse.io/v1alpha1
  kind: L2LoadBalancer
  metadata:
    name: ingress
  spec:
    addressPool:
    - 192.168.2.100-192.168.2.150
    nodeSelector:
      node-role.kubernetes.io/loadbalancer: "" # селектор узлов-балансировщиков
  ```

* Создайте ресурс _Service_ со специальными аннотациями:

  ```yaml
  apiVersion: v1
  kind: Service
  metadata:
    name: nginx-deployment
    annotations:
      network.deckhouse.io/l2-load-balancer-name: ingress
      network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
  spec:
    type: LoadBalancer
    loadBalancerClass: l2-class
    ports:
    - port: 8000
      protocol: TCP
      targetPort: 80
    selector:
      app: nginx
  ```

  В результате, созданному сервису с типом `LoadBalancer` будут присвоены адреса в заданном количестве:

  ```shell
  $ kubectl get svc
  NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)        AGE
  nginx-deployment       LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30544/TCP   11s
  ```

  Полученные EXTERNAL-IP можно прописывать в качестве A-записей для прикладного домена.

  ```shell
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.2.100:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.2.101:8000
  200
  $ curl -s -o /dev/null -w "%{http_code}" 192.168.2.102:8000
  200
  ```

{% endraw %}
