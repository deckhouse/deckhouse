---
title: "Стандартный NetworkPolicy Kubernetes"
permalink: ru/admin/configuration/network/policy/kubernetes_networkpolicy.html
description: |
  Описание модели NetworkPolicy Kubernetes, селекторов, default-политик и ограничений API в Deckhouse Kubernetes Platform.
lang: ru
relatedLinks:
  - title: "Network Policies — документация Kubernetes"
    url: https://kubernetes.io/docs/concepts/services-networking/network-policies/
  - title: "Kube-router: Enforcing Kubernetes network policies with iptables and ipset"
    url: https://cloudnativelabs.github.io/post/2017-05-1-kube-network-policies/
  - title: "Типовые примеры политик"
    url: examples.html
  - title: "Диагностика и наблюдаемость политик"
    url: troubleshooting.html
---

Стандартный ресурс [NetworkPolicy](https://kubernetes.io/docs/concepts/services-networking/network-policies/) (`networking.k8s.io/v1`) описывает правила фильтрации трафика подов на уровнях L3 и L4 (TCP, UDP, опционально SCTP). В DKP такие политики обрабатываются модулем [`cni-cilium`](/modules/cni-cilium/) или модулем [`network-policy-engine`](/modules/network-policy-engine/) — в зависимости от [выбранного CNI](configuration.html#реализация-сетевых-политик-в-dkp).

## Модель изоляции

NetworkPolicy описывает разрешения: правил deny не предусмотрено. По умолчанию под не изолирован — разрешён весь входящий и исходящий трафик. Под становится изолированным, как только хотя бы одна политика выбирает его в `spec.podSelector` и указывает соответствующее направление в `spec.policyTypes`:

- Под изолируется по входящему трафику, если на него действует политика с `policyTypes: [Ingress]`. В этом случае разрешён только трафик, описанный в `ingress`.
- Под изолируется по исходящему трафику, если на него действует политика с `policyTypes: [Egress]`. В этом случае разрешён только трафик, описанный в `egress`.
- Ответный трафик для разрешённых соединений всегда разрешён неявно.

Политики аддитивны: если к поду применяется несколько политик, итоговый набор разрешений — объединение всех правил. Порядок применения политик не влияет на результат.

Чтобы соединение между подом-источником и подом-получателем установилось, его одновременно должны разрешать `egress`-правила источника и `ingress`-правила получателя. Если хотя бы одна сторона запрещает, соединение не установится.

## Селекторы и поля

В одной политике используются следующие селекторы и поля:

- `spec.podSelector` — обязательное поле. Определяет, к каким подам применяется политика. Политика действует на поды в том же неймспейсе, в котором она создана. Пустой селектор `{}` выбирает все поды неймспейса.
- `spec.policyTypes` — список из `Ingress`, `Egress` или обоих значений. Если параметр не указан, значение `Ingress` устанавливается всегда, `Egress` — только если политика содержит правила egress.
- `ingress[].from` и `egress[].to` — комбинации источников и получателей (см. ниже).
- `ingress[].ports` и `egress[].ports` — список протоколов и портов; поддерживается одиночный порт (`port`) и диапазон через `endPort`.

В блоках `from` и `to` доступны четыре типа селекторов:

- `podSelector` — поды в том же неймспейсе, что и политика;
- `namespaceSelector` — все поды в выбранных неймспейсах;
- `podSelector` и `namespaceSelector` в одном элементе — поды с указанными лейблами в неймспейсах с указанными лейблами;
- `ipBlock` — диапазон CIDR с возможным исключением через `except`. Используется для адресов вне кластера, так как IP-адреса подов эфемерны.

Полный пример политики, использующей все основные конструкции:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: db-access
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: db
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - ipBlock:
            cidr: 172.17.0.0/16
            except:
              - 172.17.1.0/24
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
        - podSelector:
            matchLabels:
              role: backend
      ports:
        - protocol: TCP
          port: 6379
  egress:
    - to:
        - ipBlock:
            cidr: 10.0.0.0/24
      ports:
        - protocol: TCP
          port: 5978
```

Эта политика применяется к подам с лейблом `app: db` в неймспейсе `my-app`. Для входящего трафика разрешены подключения на TCP-порт 6379 от трёх источников: из CIDR `172.17.0.0/16` за исключением `172.17.1.0/24`, из любых подов в неймспейсе `frontend`, из подов с лейблом `role: backend` в локальном неймспейсе. Для исходящего трафика разрешены подключения только на TCP-порт 5978 в подсеть `10.0.0.0/24`.

### Различие между объединением и пересечением селекторов

Один элемент массива `from`/`to` с двумя селекторами означает «И» (пересечение):

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            user: alice
        podSelector:
          matchLabels:
            role: client
```

Здесь разрешены соединения от подов с лейблом `role=client` из неймспейса с лейблом `user=alice`.

Два отдельных элемента массива означают «ИЛИ» (объединение):

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            user: alice
      - podSelector:
          matchLabels:
            role: client
```

Здесь разрешены соединения от подов с лейблом `role=client` в локальном неймспейсе (в том же, где создан ресурс NetworkPolicy) или от любых подов в неймспейсе с лейблом `user=alice`.

### Выбор неймспейса по имени

Поле для прямого указания имени неймспейса в спецификации отсутствует. Вместо этого используйте лейбл `kubernetes.io/metadata.name`. Kubernetes автоматически устанавливает этот лейбл на каждый неймспейс, а его значение равно имени неймспейса.

```yaml
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            kubernetes.io/metadata.name: frontend
```

### Диапазон портов

Через поля `port` и `endPort` можно описать диапазон портов:

```yaml
egress:
  - to:
      - ipBlock:
          cidr: 10.0.0.0/24
    ports:
      - protocol: TCP
        port: 32000
        endPort: 32768
```

Ограничения: `endPort` должен быть больше или равен `port`; `endPort` указывается только вместе с `port`; оба значения должны быть числовыми.

## Политики по умолчанию для неймспейса

Для неймспейса применяют типовые политики по умолчанию, задавая базовое поведение:

- запретить весь входящий трафик к подам неймспейса:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: default-deny-ingress
  spec:
    podSelector: {}
    policyTypes:
      - Ingress
  ```

- запретить весь исходящий трафик из подов неймспейса:

  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: NetworkPolicy
  metadata:
    name: default-deny-egress
  spec:
    podSelector: {}
    policyTypes:
      - Egress
  ```

- запретить и входящий, и исходящий трафик: указать оба значения в `policyTypes` без правил.
- разрешить весь входящий или исходящий трафик: добавить пустое правило `ingress: [{}]` или `egress: [{}]` соответственно.

{% alert level="warning" %}
Default-политика deny-egress блокирует и DNS-запросы. Если поды используют DNS, добавьте отдельную политику, разрешающую egress на сервис kube-dns в `kube-system` (порты UDP/53 и TCP/53).
{% endalert %}

## Поведение в особых случаях

### Поды в режиме `hostNetwork`

Поведение NetworkPolicy для подов с `hostNetwork: true` не определено в API. Большинство движков, включая Cilium и kube-router, не различают трафик таких подов и трафик самого узла. Селекторы `podSelector` и `namespaceSelector` к ним не применяются, а трафик считается трафиком узла. Для фильтрации используйте `ipBlock` с IP-адресом узла или [host firewall на узлах](host_firewall.html).

### Жизненный цикл подов

После создания NetworkPolicy движок применяет её асинхронно. Под, выбранный политикой, может в первые секунды стартовать без правил изоляции или с частично применёнными правилами. Для критических зависимостей используйте init-контейнеры, ожидающие доступности нужных адресов.

### Существующие соединения

Поведение при изменении набора политик во время уже установленного соединения не определено: одни движки разрывают такое соединение, другие оставляют до закрытия. Не меняйте политики и лейблы подов или неймспейсов в моменты, когда это может затронуть рабочий трафик.

### L4-фильтрация

NetworkPolicy определена для соединений L4 (TCP, UDP, опционально SCTP). Поведение для других протоколов (например, ICMP, ARP) зависит от движка и может отличаться.

## Реализация без Cilium: модуль `network-policy-engine`

Если в кластере не используется Cilium, политики обрабатывает модуль [`network-policy-engine`](/modules/network-policy-engine/) на базе [kube-router](https://github.com/cloudnativelabs/kube-router). Модуль разворачивает в неймспейсе `d8-system` DaemonSet, в котором kube-router работает в режиме Network Policy Controller и [транслирует политики в правила `iptables` и `ipset`](https://cloudnativelabs.github.io/post/2017-05-1-kube-network-policies/) на каждом узле:

- для каждой политики создаётся отдельная цепочка `KUBE-NWPLCY-*`;
- для каждого пода, попадающего под изоляцию, создаётся цепочка `KUBE-POD-SPECIFIC-FW-*`;
- IP-адреса подов-источников и получателей хранятся в ipset, что позволяет компактно описывать большие наборы и быстро применять изменения.

Поддерживаются только стандартные форматы Kubernetes: `networking.k8s.io/NetworkPolicy API`, V1/GA и beta-семантика. Расширений Cilium (CiliumNetworkPolicy, CiliumClusterwideNetworkPolicy, L7-правила, FQDN, deny-правила) этот движок не поддерживает.

Готовые примеры стандартных политик, работающих и через `network-policy-engine`, и через `cni-cilium`, представлены в разделе [«Типовые примеры политик»](examples.html).

## Ограничения API

API NetworkPolicy Kubernetes не поддерживает следующие сценарии (это ограничения API, а не конкретного движка):

- правила уровня L7 (HTTP, gRPC, Kafka, DNS-фильтрация по именам);
- правила deny — модель политики «default deny + явные allow»;
- выбор сервисов по имени;
- маршрутизация всего трафика через общий gateway;
- правила, привязанные к конкретным узлам по их Kubernetes-идентичности (только через `ipBlock` с IP-адресами узлов);
- логирование событий — какие соединения разрешены или запрещены;
- запрет loopback-трафика и трафика от собственного узла пода.

Часть этих задач решается через [CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html), доступные в кластерах с `cni-cilium`.
