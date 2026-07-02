---
title: "Типовые примеры сетевых политик"
permalink: ru/admin/configuration/network/policy/examples.html
description: |
  Готовые рецепты сетевых политик для Deckhouse Kubernetes Platform: запрет трафика в namespace, разрешения по namespace и подам, egress к DNS, доступ к API-серверу, правила L7 и FQDN.
lang: ru
relatedLinks:
  - title: "Стандартный NetworkPolicy Kubernetes"
    url: kubernetes_networkpolicy.html
  - title: "CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy"
    url: cilium_networkpolicy.html
  - title: "Host firewall на узлах"
    url: host_firewall.html
  - title: "Диагностика и наблюдаемость политик"
    url: troubleshooting.html
---

В этом разделе собраны типовые сценарии настройки сетевых политик. Стандартные примеры (NetworkPolicy) работают в любых кластерах с поддержкой политик; примеры с CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy — только в кластерах с модулем [`cni-cilium`](/modules/cni-cilium/).

Для конструкций самих ресурсов используйте описание из [Kubernetes NetworkPolicy](kubernetes_networkpolicy.html) и [CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html).

## Запретить весь входящий трафик в неймспейс, но разрешить взаимодействие внутри него

Подходит как baseline для неймспейса, в котором поды должны взаимодействовать друг с другом, но не быть доступны извне:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-external-ingress
  namespace: my-app
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector: {}
  egress:
    - {}
```

Все egress-соединения остаются разрешёнными благодаря пустому правилу `egress: [{}]`; ingress разрешён только от подов того же неймспейса.

## Разрешить входящий трафик из конкретного неймспейса

Разрешает подам из неймспейса с лейблом `kubernetes.io/metadata.name: frontend` обращаться к подам с лейблом `app: api`:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend-to-api
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: api
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
      ports:
        - protocol: TCP
          port: 8080
```

Лейбл `kubernetes.io/metadata.name` Kubernetes устанавливает на каждый неймспейс автоматически.

## Разрешить исходящий трафик только к DNS и заданному CIDR

Default-deny egress + точечное разрешение DNS и одного внешнего сервиса:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: restrict-egress
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: client
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    - to:
        - ipBlock:
            cidr: 10.0.0.0/24
      ports:
        - protocol: TCP
          port: 5432
```

## Разрешить доступ к API-серверу из конкретных подов

Через сущность `kube-apiserver` Cilium самостоятельно отслеживает IP-адреса API-сервера, поэтому правило не нужно обновлять при смене адресации:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-controller-to-apiserver
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: controller
  egress:
    - toEntities:
        - kube-apiserver
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

## Разрешить только GET-запросы к API на L7

Клиентам с лейблом `app: client` разрешён только `GET /api/v1/...` к подам с лейблом `app: api`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: readonly-api
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: client
      toPorts:
        - ports:
            - port: "8080"
              protocol: TCP
          rules:
            http:
              - method: GET
                path: "/api/v1/.*"
```

## Разрешить egress только к указанным DNS-именам (FQDN)

Для FQDN-правил Cilium должен видеть DNS-запросы, поэтому в той же политике обязательно разрешите исходящий трафик на DNS-сервер кластера с инспекцией DNS:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: egress-to-fqdns
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toEntities:
        - cluster
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
          rules:
            dns:
              - matchPattern: "*"
    - toFQDNs:
        - matchName: "api.example.com"
        - matchPattern: "*.cdn.example.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
```

{% alert level="info" %}
В DNS-правиле используется `toEntities: cluster`, а не селектор по лейблам `kube-dns`. В DKP наряду с основным DNS-сервисом работает DaemonSet `node-local-dns`, поэтому реальный путь DNS-трафика от пода может проходить через локальный экземпляр `node-local-dns`. Использование `toEntities: cluster` надёжно покрывает любой DNS-эндпоинт внутри кластера.
{% endalert %}

## Запретить обращения к metadata-сервису облака

Deny-правило на cluster-scope блокирует доступ ко всему сервису метаданных облачного провайдера для любого пода кластера, даже если другие политики разрешают подобный egress:

```yaml
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: deny-egress-to-metadata
spec:
  endpointSelector: {}
  egressDeny:
    - toCIDR:
        - 169.254.169.254/32
```

