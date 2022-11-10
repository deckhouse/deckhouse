---
title: "The metallb module: пример"
---

{% raw %}

Metallb можно использовать в статических (Bare Metal) кластерах, когда вы не можете заказать load balancer у облачного провайдера. Metallb может работать в L2 или BGP режиме.

Ниже представлен пример использования Metallb в L2 режиме.
Мы создадим Ingress Controller с inlet `LoadBalancer`. А также дадим доступ к отдельно запущенному Nginx, используя сервис с типом `LoadBalancer`.

Во-первых, необходимо определить, какие NodeGroup'ы будут использоваться для запуска приложений, к которым будет предоставлен доступ.
В этом примере Ingress-контроллеры запускаются на frontend-узлах, а Nginx — на worker-узле. У всех узлов есть общий лейбл `node-role/metallb=""`.

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

Проверьте, что на узлах проставлен корректный лейбл.

```bash
kubectl get nodes -l node-role/metallb
NAME              STATUS   ROLES      AGE   VERSION
demo-frontend-0   Ready    frontend   61d   v1.21.14
demo-frontend-1   Ready    frontend   61d   v1.21.14
demo-worker-0     Ready    worker     61d   v1.21.14
```

Модуль `metallb` отключен по умолчанию, поэтому его необходимо включить явно. Также необходимо задать корректные `nodeSelector` и `tolerations` для Metallb speaker'ов.

```yaml
metallb: |
  addressPools:
  - addresses:
    - 192.168.199.100-192.168.199.102
    name: frontend_pool
    protocol: layer2
  speaker:
    nodeSelector:
      node-role/metallb: ""
    tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      operator: Equal
      value: frontend
metallbEnabled: "true"
```

Создайте `IngressNginxController`.

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

Проверьте, что сервис с типом `LoadBalancer` создан в namespace `d8-ingress-nginx`.

```shell
kubectl -n d8-ingress-nginx get svc main-load-balancer 
NAME                 TYPE           CLUSTER-IP       EXTERNAL-IP       PORT(S)                      AGE
main-load-balancer   LoadBalancer   10.222.255.194   192.168.199.100   80:30236/TCP,443:32292/TCP   30s
```

Ваш Ingress-контроллер доступен по внешнему IP-адресу.

```shell
curl -s -o /dev/null -w "%{http_code}" 192.168.199.100
404
```

Теперь предоставим доступ к Nginx на порту `8080`.

```shell
kubectl create deploy nginx --image=nginx
kubectl create svc loadbalancer nginx --tcp=8080:80
```

Проверьте сервис.

```shell
kubectl get svc nginx
NAME    TYPE           CLUSTER-IP     EXTERNAL-IP       PORT(S)          AGE
nginx   LoadBalancer   10.222.9.190   192.168.199.101   8080:31689/TCP   3m11s
```

Теперь вы можете получить доступ к приложению, используя curl.

```shell
curl -s -o /dev/null -w "%{http_code}" 192.168.199.101:8080
200
```

{% endraw %}
