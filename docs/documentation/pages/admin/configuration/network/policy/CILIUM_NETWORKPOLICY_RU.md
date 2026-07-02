---
title: "CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy"
permalink: ru/admin/configuration/network/policy/cilium_networkpolicy.html
description: |
  Описание расширений Cilium для сетевых политик в Deckhouse Kubernetes Platform: entities, правила L7, FQDN-правила, deny-правила, режим policyAuditMode.
lang: ru
relatedLinks:
  - title: "Network Policy — документация Cilium"
    url: https://docs.cilium.io/en/v1.17/network/kubernetes/policy/
  - title: "Overview of Network Policy — документация Cilium"
    url: https://docs.cilium.io/en/v1.17/security/policy/
  - title: "Host firewall на узлах"
    url: host_firewall.html
  - title: "Типовые примеры политик"
    url: examples.html
  - title: "Диагностика и наблюдаемость политик"
    url: troubleshooting.html
---

В кластерах с включённым модулем [`cni-cilium`](/modules/cni-cilium/) дополнительно к стандартному NetworkPolicy доступны два формата от Cilium:

- CiliumNetworkPolicy — namespaced-ресурс с правилами уровней L3–L7;
- CiliumClusterwideNetworkPolicy — cluster-scoped-ресурс с теми же правилами и поддержкой `nodeSelector`.

Cilium может одновременно применять политики всех трёх форматов.

{% alert level="warning" %}
При одновременном использовании NetworkPolicy, CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy итоговый набор разрешённого трафика становится сложнее анализировать. Используйте режим аудита перед применением и проверяйте поведение в Hubble.
{% endalert %}

## Порядок обработки правил

При объединении правил из NetworkPolicy, CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy Cilium использует следующие принципы:

- deny-правила имеют приоритет над allow-правилами;
- allow-правила из NetworkPolicy, CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy объединяются;
- если для эндпоинта (пода) существует хотя бы одна политика, начинает действовать модель default-deny для соответствующего направления трафика;
- L7-правила применяются после прохождения проверок уровней L3/L4.

## Что добавляют CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy

По сравнению со стандартным NetworkPolicy:

- правила уровней L7 — протоколы HTTP, gRPC, Kafka и DNS;
- FQDN-правила в egress — фильтрация по DNS-именам;
- deny-правила — явный запрет соединений;
- сущности (`entities`) — источники и получатели трафика, например `kube-apiserver`, `host`, `remote-node`, `world`;
- ссылки на Kubernetes-сервисы по имени или лейблам (`toServices`) — разрешения на egress без указания CIDR;
- фильтрация ICMP и ICMPv6 по типу пакета;
- фильтрация TLS по Server Name Indication (SNI);
- `nodeSelector` (только в CiliumClusterwideNetworkPolicy) — применение правила к самим узлам; это даёт основу для [host firewall на узлах](host_firewall.html);
- режим аудита через [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) — логирование вердиктов без блокировки.

## Структура ресурса

CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy описываются единым набором полей `spec`. Минимальная структура:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: example
  namespace: default
spec:
  endpointSelector:
    matchLabels:
      app: db
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: frontend
      toPorts:
        - ports:
            - port: "5432"
              protocol: TCP
```

Ключевые поля:

- `endpointSelector` — выбор подов, к которым применяется политика. Аналог `podSelector` в стандартном NetworkPolicy.
- `nodeSelector` — выбор узлов (только в CiliumClusterwideNetworkPolicy). В одной политике может быть указан либо `endpointSelector`, либо `nodeSelector`.
- `ingress` и `egress` — массивы правил. Каждое правило содержит источник или получатель (`fromEndpoints`, `fromEntities`, `fromCIDR`, `fromCIDRSet`, `toEndpoints`, `toEntities`, `toCIDR`, `toCIDRSet`, `toFQDNs`, `toServices`) и опциональный фильтр портов и протоколов в `toPorts`.
- `ingressDeny` и `egressDeny` — deny-правила. Применяются раньше allow-правил.

При использовании селекторов пригодятся два специальных лейбла, которые Cilium автоматически проставляет на эндпоинт каждого пода:

- `io.kubernetes.pod.namespace` — имя неймспейса, в котором запущен под. Используется в `fromEndpoints` и `toEndpoints` для ссылки на поды из конкретного namespace.
- `k8s-app`, `app`, обычные лейблы пода — доступны без префикса.

### Egress на Kubernetes-сервис

Поле `toServices` позволяет описать egress на Kubernetes-сервис, а не на группу подов. Сервис выбирают по имени и неймспейсу (`k8sService`) либо по лейблам (`k8sServiceSelector`):

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-egress-to-redis
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toServices:
        - k8sService:
            serviceName: redis
            namespace: data
```

Политика автоматически отслеживает изменения бэкендов сервиса и применяет к ним соответствующие правила.

## Сущности (entities)

Сущности — это источники и получатели трафика, по которым удобно описывать трафик до системных компонентов кластера и инфраструктуры:

- `host` — собственный узел пода, включая трафик хоста;
- `remote-node` — другие узлы кластера;
- `kube-apiserver` — API-сервер Kubernetes (используется в host firewall);
- `cluster` — все поды и узлы кластера;
- `world` — всё за пределами кластера;
- `health` — health-эндпоинты Cilium;
- `init` — контейнеры до получения identity;
- `unmanaged` — поды без управления Cilium;
- `all` — любая сущность.

Пример ingress-правила, разрешающего обращения от API-сервера (сущность `kube-apiserver`) к подам с лейблом `app: webhook`:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-apiserver-to-webhook
  namespace: default
spec:
  endpointSelector:
    matchLabels:
      app: webhook
  ingress:
    - fromEntities:
        - kube-apiserver
      toPorts:
        - ports:
            - port: "9443"
              protocol: TCP
```

## Правила L4: ICMP и SNI

Помимо ограничения по портам и протоколам, в `toPorts` доступны дополнительные L4-фильтры.

### ICMP и ICMPv6

Поле `icmps` разрешает или запрещает ICMP-сообщения по типу пакета:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-icmp-echo
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: probe
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: monitoring
      icmps:
        - fields:
            - type: EchoRequest
              family: IPv4
```

Без явного `icmps` в политике с активной L4-фильтрацией ICMP-трафик блокируется наравне с TCP и UDP.

### TLS Server Name Indication (SNI)

В egress можно ограничить трафик по SNI — имени, которое клиент передаёт в TLS ClientHello. Это позволяет фильтровать обращения к внешним HTTPS-сервисам:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-egress-tls-sni
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: client
  egress:
    - toFQDNs:
        - matchPattern: "*.example.com"
      toPorts:
        - ports:
            - port: "443"
              protocol: TCP
          serverNames:
            - "api.example.com"
            - "static.example.com"
```

Если `serverNames` указан, разрешены только TLS-соединения с перечисленными именами; обращения с другим SNI блокируются на уровне TLS-handshake.

## Правила L7

CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy позволяют описать разрешённые операции уровня приложения. L7-правила указывают внутри `toPorts[].rules`:

{% alert level="warning" %}
L7-инспекция выполняется через прокси Envoy в составе агента Cilium на каждом узле. Это добавляет задержку на каждое обрабатываемое соединение и увеличивает нагрузку на CPU узла. Не используйте L7-правила на горячих путях, если в этом нет необходимости — для базовой фильтрации достаточно правил уровней L3 и L4.
{% endalert %}

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-readonly-api
  namespace: default
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

Здесь клиентам с лейблом `app: client` разрешены только запросы `GET /api/v1/...` к подам с лейблом `app: api` на порт 8080.

Поддерживаются протоколы HTTP, gRPC, Kafka и DNS. Подробности и ограничения описаны в разделе [Layer 7 Examples документации Cilium](https://docs.cilium.io/en/v1.17/security/policy/#layer-7-examples).

### Kafka

Для Kafka L7-правила разрешают конкретные операции (`apiKey`) и топики:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-kafka-produce
  namespace: my-app
spec:
  endpointSelector:
    matchLabels:
      app: kafka
  ingress:
    - fromEndpoints:
        - matchLabels:
            app: producer
      toPorts:
        - ports:
            - port: "9092"
              protocol: TCP
          rules:
            kafka:
              - role: produce
                topic: orders
```

Подам с лейблом `app: producer` разрешена только публикация (`role: produce`) в топик `orders`. Любые другие операции, включая `consume` и `metadata`, будут отклонены на уровне протокола Kafka.

### DNS Policy и IP Discovery

При использовании `toFQDNs` Cilium перехватывает DNS-ответы, разрешённые правилом `rules.dns`, и обновляет внутренний кеш сопоставлений DNS-имя → IP-адрес. Этот кеш и используется при принятии решений по `toFQDNs`-правилам. Поэтому:

{% alert level="warning" %}
Когда `toFQDNs` сочетается с DNS-инспекцией через `rules.dns`, DNS-запрос приложения проходит через прокси Cilium на узле — фактически удваивается путь резолва. На больших объёмах DNS-трафика это заметно увеличивает задержку и нагрузку на cilium-agent. Сужайте `matchPattern` в `rules.dns` до необходимого минимума.
{% endalert %}

- DNS-egress должен идти в той же политике, что и FQDN-правило, или в любой другой политике, выбирающей те же поды;
- если поду запрещены DNS-запросы, его FQDN-правила работать не будут;
- TTL и срок жизни записей в кеше определяет агент Cilium на основе DNS-ответов.

## FQDN-правила

В egress можно ограничить трафик по DNS-именам через `toFQDNs`. Для работы `toFQDNs` необходимо разрешить DNS-запросы и включить DNS-инспекцию через `rules.dns`. Это можно сделать в той же политике или в другой политике, которая выбирает те же поды:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-egress-to-example
  namespace: default
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

## Deny-правила

В отличие от стандартного NetworkPolicy, в CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy можно явно запретить трафик, не отменяя при этом более широкие allow-правила:

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

Deny-правила применяются раньше allow-правил, поэтому они приоритетнее любых разрешений из других политик.

## Политики по умолчанию через CiliumNetworkPolicy

Как и в стандартном NetworkPolicy, наличие политики с пустыми списками правил переводит выбранные эндпоинты в режим deny по умолчанию.

Чтобы перевести неймспейс в режим default-deny, создайте CiliumNetworkPolicy с пустым списком правил:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: default-deny
  namespace: secure
spec:
  endpointSelector: {}
  ingress: []
  egress: []
```

DNS тоже попадает под действие политик, необходимо явно разрешить egress-запросы к DNS-сервису кластера через UDP/53 и TCP/53:

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-dns
  namespace: secure
spec:
  endpointSelector: {}
  egress:
    - toEntities:
        - cluster
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
```

{% alert level="info" %}
В DNS-правиле используется `toEntities: cluster`, а не селектор по лейблам `kube-dns`. В DKP наряду с основным DNS-сервисом работает DaemonSet `node-local-dns`, поэтому реальный путь DNS-трафика от пода может проходить через локальный экземпляр `node-local-dns`. Использование `toEntities: cluster` надёжно покрывает любой DNS-эндпоинт внутри кластера.
{% endalert %}

## Режим аудита (`policyAuditMode`)

Параметр модуля [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) переводит Cilium в режим, в котором политики не блокируют трафик, а только логируют вердикты. Это позволяет безопасно внедрять большие наборы политик и проверять их в Hubble UI до окончательного включения.

{% alert level="warning" %}
В режиме аудита никакая сетевая политика не блокирует трафик. Не используйте режим аудита как постоянную настройку, оставляйте его включённым только на время внедрения.
{% endalert %}

Рекомендуемый порядок действий:

1. Включите параметр `policyAuditMode: true` в [настройках модуля `cni-cilium`](/modules/cni-cilium/configuration.html#parameters-policyauditmode).
1. Примените набор политик. Политики для узлов применяйте отдельно по процедуре из раздела [Host firewall на узлах](host_firewall.html).
1. Проверьте вердикты в Hubble UI и через `hubble observe --type policy-verdict`. В выводе должны появиться записи `verdict=AUDITED` для соединений, которые без режима аудита были бы заблокированы.
1. Доработайте политики до того момента, пока в логе не останется только записей `verdict=ALLOWED` и `verdict=AUDITED` для ожидаемых сценариев.
1. Отключите режим аудита (`policyAuditMode: false`).

После отключения режима аудита политики начнут блокировать трафик, не подходящий ни под одно allow-правило.

