---
title: "The metallb module: примеры"
---

Metallb можно использовать в статических кластерах (bare metal), когда нет возможности воспользоваться балансировщиком от облачного провайдера. Metallb может работать в режимах L2 или BGP.

## Пример использования metallb в L2-режиме

{% raw %}
Пример включения модуля metallb и публикации отдельно запущенного приложения (веб-сервер Nginx).

1. Задайте группы узлов ([_NodeGroup_](../040-node-manager/cr.html#nodegroup)) для запуска приложений, к которым предоставляется доступ.

   Например, Ingress-контроллеры запускаются на frontend-узлах, а веб-сервер Nginx — на worker-узле. У frontend-узлов есть лейбл `node-role.deckhouse.io/metallb=""`.

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
         node-role.deckhouse.io/metallb: ""
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
     nodeType: Static
   ```

1. Проверьте, что на узлах проставлен корректный лейбл:

   ```bash
   kubectl get nodes -l node-role.deckhouse.io/metallb
   ```

   Пример вывода:

   ```bash
   $ kubectl get nodes -l node-role.deckhouse.io/metallb
   NAME              STATUS   ROLES      AGE   VERSION
   demo-frontend-0   Ready    frontend   61d   v1.21.14
   demo-frontend-1   Ready    frontend   61d   v1.21.14
   ```

1. Включите модуль metallb и задайте параметры `nodeSelector` и `tolerations` для спикеров MetalLB.

   Пример конфигурации модуля:

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
           node-role.deckhouse.io/metallb: ""
         tolerations:
         - effect: NoExecute
           key: dedicated.deckhouse.io
           operator: Equal
           value: frontend
   ```

1. Установите приложение (nginx) и опубликуйте на порту `8080`:

   ```shell
   kubectl create deploy nginx --image=nginx
   kubectl create svc loadbalancer nginx --tcp=8080:80
   ```

1. Проверьте, что сервис создан:

   ```shell
   kubectl get svc nginx
   ```

   Пример вывода:

   ```shell
   $ kubectl get svc nginx
   NAME    TYPE           CLUSTER-IP     EXTERNAL-IP       PORT(S)          AGE
   nginx   LoadBalancer   10.222.9.190   192.168.199.101   8080:31689/TCP   3m11s
   ```

1. Проверьте доступ к приложению.

   Пример:

   ```console
   $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.101:8080
   200
   ```

{% endraw %}

## Публикация сервиса через L2LoadBalancer

{% raw %}

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
