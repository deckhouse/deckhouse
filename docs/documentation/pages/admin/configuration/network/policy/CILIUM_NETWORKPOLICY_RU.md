---
title: "CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy"
permalink: ru/admin/configuration/network/policy/cilium_networkpolicy.html
description: |
  Описание расширений Cilium для сетевых политик в Deckhouse Kubernetes Platform: entities, правила L7, FQDN-правила, deny-правила, режим policyAuditMode.
lang: ru
---

В кластерах с модулем [`cni-cilium`](/modules/cni-cilium/) дополнительно к стандартному `NetworkPolicy` доступны два формата от Cilium:

- `CiliumNetworkPolicy` (CNP) — namespaced-ресурс с правилами уровней L3–L7;
- `CiliumClusterwideNetworkPolicy` (CCNP) — cluster-scoped-ресурс с теми же правилами и поддержкой `nodeSelector`.

Cilium может одновременно применять политики всех трёх форматов.

{% alert level="warning" %}
При одновременном использовании `NetworkPolicy`, CNP и CCNP итоговый набор разрешённого трафика становится сложнее анализировать. Используйте режим аудита перед выкаткой и проверяйте поведение в Hubble.
{% endalert %}

## Что добавляют CNP и CCNP

По сравнению со стандартным `NetworkPolicy`:

- правила уровней L7 — HTTP, gRPC, Kafka и DNS;
- FQDN-правила в egress — фильтрация по DNS-именам;
- deny-правила — явный запрет соединений;
- сущности (`entities`) — встроенные группы получателей и отправителей, например `kube-apiserver`, `host`, `remote-node`, `world`;
- `nodeSelector` (только в CCNP) — применение правила к самим узлам, что позволяет реализовать host firewall (см. [Host firewall на узлах](host_firewall.html));
- режим аудита через [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) — логирование вердиктов без блокировки.

## Структура ресурса

CNP и CCNP описываются единым набором полей `spec`. Минимальная структура:

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

- `endpointSelector` — выбор подов, к которым применяется политика. Аналог `podSelector` в стандартном `NetworkPolicy`.
- `nodeSelector` — выбор узлов (только в CCNP). Не используется одновременно с `endpointSelector` в одной политике.
- `ingress` и `egress` — массивы правил. Каждое правило содержит источник или получатель (`fromEndpoints`, `fromEntities`, `fromCIDR`, `fromCIDRSet`, `toEndpoints`, `toEntities`, `toCIDR`, `toCIDRSet`, `toFQDNs`, `toServices`) и опциональный фильтр портов и протоколов в `toPorts`.
- `ingressDeny` и `egressDeny` — deny-правила. Применяются раньше allow-правил.

## Сущности (entities)

Сущности — это встроенные группы получателей и отправителей, по которым удобно описывать трафик до системных компонентов кластера и инфраструктуры:

- `host` — собственный узел пода, включая трафик хоста;
- `remote-node` — другие узлы кластера;
- `kube-apiserver` — API-сервер Kubernetes (используется в host firewall);
- `cluster` — все поды и узлы кластера;
- `world` — всё за пределами кластера;
- `health` — health-эндпоинты Cilium;
- `init` — контейнеры до получения identity;
- `unmanaged` — поды без управления Cilium;
- `all` — любая сущность.

Пример ingress-правила, разрешающего обращения от API-сервера к подам с лейблом `app: webhook`:

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

## Правила L7

CNP и CCNP позволяют описать разрешённые операции уровня приложения. L7-правила указывают внутри `toPorts[].rules`:

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

Поддерживаются протоколы HTTP, gRPC, Kafka и DNS. Подробности и ограничения — в разделе [Layer 7 Examples документации Cilium](https://docs.cilium.io/en/v1.17/security/policy/#layer-7-examples).

## FQDN-правила

В egress можно ограничить трафик по DNS-именам через `toFQDNs`. Чтобы Cilium успевал обновлять разрешённые IP-адреса, в той же политике обязательно разрешите DNS-запросы и включите DNS-инспекцию через `rules.dns`:

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
    - toEndpoints:
        - matchLabels:
            io.kubernetes.pod.namespace: kube-system
            k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
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

## Deny-правила

В отличие от стандартного `NetworkPolicy`, в CNP и CCNP можно явно запретить трафик, не отменяя при этом более широкие allow-правила:

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

## Default-политики через CNP

Чтобы перевести namespace в режим default-deny, создайте CNP с пустым списком правил:

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

Если поды используют DNS, дополнительно разрешите egress на kube-dns через UDP/53 и TCP/53.

## Режим аудита (`policyAuditMode`)

Параметр модуля [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode) переводит Cilium в режим, в котором политики не блокируют трафик, а только логируют вердикты. Это позволяет безопасно выкатывать большие наборы политик и проверять их в Hubble UI до окончательного включения.

{% alert level="warning" %}
В режиме аудита **никакая** сетевая политика не блокирует трафик. Не используйте режим аудита как постоянную настройку, оставляйте его включённым только на время выкатки.
{% endalert %}

Рекомендуемый порядок:

1. Включите параметр `policyAuditMode: true` в [настройках модуля `cni-cilium`](/modules/cni-cilium/configuration.html#parameters-policyauditmode).
2. Примените набор политик. Не применяйте host-политики до проверки (см. [Host firewall на узлах](host_firewall.html)).
3. Проверьте вердикты в Hubble UI и через `hubble observe --type policy-verdict`. В выводе должны появиться записи `verdict=AUDITED` для соединений, которые без режима аудита были бы заблокированы.
4. Доработайте политики до того момента, пока в логе не останется только записей `verdict=ALLOWED` и `verdict=AUDITED` для ожидаемых сценариев.
5. Отключите режим аудита (`policyAuditMode: false`).

После отключения режима аудита политики начнут блокировать трафик, не подходящий ни под одно allow-правило.

## См. также

- [Network Policy — документация Cilium](https://docs.cilium.io/en/v1.17/network/kubernetes/policy/)
- [Overview of Network Policy — документация Cilium](https://docs.cilium.io/en/v1.17/security/policy/)
- [Host firewall на узлах](host_firewall.html)
- [Типовые примеры политик](examples.html)
- [Диагностика и наблюдаемость политик](troubleshooting.html)
