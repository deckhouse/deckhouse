---
title: "The metallb module: примеры"
---

Metallb можно использовать в статических кластерах (bare metal), когда облачный провайдер не предоставляет балансировщик (load balancer). Metallb может работать в L2- или BGP-режиме.

## Пример использования metallb в L2-режиме

{% raw %}
Пример включения модуля metallb, создания Ingress-контроллера с `inlet: LoadBalancer` и предоставления доступа к отдельно запущенному веб-серверу Nginx.

1. Задайте группы узлов ([_NodeGroup_](../040-node-manager/cr.html#nodegroup)) для запуска приложений, к которым предоставляется доступ.

   Например, Ingress-контроллеры запускаются на frontend-узлах, а веб-сервер Nginx — на worker-узле. У всех узлов есть общий лейбл `node-role/metallb=""`.

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

1. Проверьте, что на узлах проставлен корректный лейбл:

   ```bash
   kubectl get nodes -l node-role/metallb
   ```

   Пример вывода:

   ```bash
   $ kubectl get nodes -l node-role/metallb
   NAME              STATUS   ROLES      AGE   VERSION
   demo-frontend-0   Ready    frontend   61d   v1.21.14
   demo-frontend-1   Ready    frontend   61d   v1.21.14
   demo-worker-0     Ready    worker     61d   v1.21.14
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
           node-role/metallb: ""
         tolerations:
         - effect: NoExecute
           key: dedicated.deckhouse.io
           operator: Equal
           value: frontend
   ```

1. Создайте кастомный ресурс _IngressNginxController_.

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

1. Проверьте, что сервис с типом `LoadBalancer` создан в _Namespace_ `d8-ingress-nginx`:

   ```shell
   kubectl -n d8-ingress-nginx get svc main-load-balancer
   ```

   Пример вывода:

   ```shell
   $ kubectl -n d8-ingress-nginx get svc main-load-balancer 
   NAME                 TYPE           CLUSTER-IP       EXTERNAL-IP       PORT(S)                      AGE
   main-load-balancer   LoadBalancer   10.222.255.194   192.168.199.100   80:30236/TCP,443:32292/TCP   30s
   ```

1. Проверьте, что Ingress-контроллер доступен по внешнему IP-адресу.

   Пример:

   ```console
   $ curl -s -o /dev/null -w "%{http_code}" 192.168.199.100
   404
   ```

1. Предоставьте доступ к веб-серверу Nginx на порту `8080`:

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
