---
title: "Диагностика и наблюдаемость политик"
permalink: ru/admin/configuration/network/policy/troubleshooting.html
description: |
  Способы проверки применённых сетевых политик в Deckhouse Kubernetes Platform: kubectl describe, Hubble UI и CLI, flow logs, чек-лист «политика не применяется».
lang: ru
---

В этом разделе собраны команды и приёмы для проверки применённых сетевых политик и расследования проблем со связностью. Часть инструментов работает только в кластерах с модулем [`cni-cilium`](/modules/cni-cilium/) — это указано отдельно.

## Проверка применённой политики

Краткое описание ресурса:

```bash
d8 k describe networkpolicy <name> -n <namespace>
d8 k describe ciliumnetworkpolicy <name> -n <namespace>
d8 k describe ciliumclusterwidenetworkpolicy <name>
```

В выводе видны выбранные поды или узлы, итоговые правила ingress и egress, а также события об ошибках валидации.

Список политик, влияющих на конкретный неймспейс:

```bash
d8 k get networkpolicy,ciliumnetworkpolicy -n <namespace>
d8 k get ciliumclusterwidenetworkpolicy
```

Какие поды попали под изоляцию и какие политики к ним применились (только в кластерах с Cilium):

```bash
d8 k -n d8-cni-cilium exec ds/agent -- cilium-dbg endpoint list
d8 k -n d8-cni-cilium exec ds/agent -- cilium-dbg endpoint get <endpoint-id>
```

В выводе `cilium-dbg endpoint list` для каждого пода-эндпоинта видны статусы `POLICY (ingress)` и `POLICY (egress)`: `Enabled`, `Disabled` или `Disabled (Audit)` в режиме [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode).

## Наблюдаемость через Hubble

Hubble отображает вердикты политик в реальном времени — это основной инструмент диагностики в кластерах с Cilium.

В Hubble UI видны соединения между подами и сервисами с пометками `forwarded`, `dropped` и `audit`. Drop-события показывают, какая политика отклонила трафик и какое поле правила сработало.

Через `hubble observe` можно фильтровать события по типу. В DKP клиент `hubble` поставляется вместе с агентом, поэтому команды удобно запускать через `d8 k exec` в под cilium-agent:

```bash
d8 k -n d8-cni-cilium exec ds/agent -- hubble observe --type policy-verdict --verdict DROPPED
d8 k -n d8-cni-cilium exec ds/agent -- hubble observe --type policy-verdict --verdict AUDITED
d8 k -n d8-cni-cilium exec ds/agent -- hubble observe --from-pod my-app/client --to-pod my-app/api
```

Каждый агент видит события только своего узла. Для общего сбора событий по кластеру используйте Hubble UI или экспорт через [`HubbleMonitoringConfig`](/modules/cni-cilium/cr.html#hubblemonitoringconfig).

В выводе указаны идентификаторы политик, селекторов и сами поля ingress/egress, которые сработали. Это позволяет быстро понять, какое именно правило блокирует или пропускает соединение.

## Сбор flow logs на постоянной основе

Для постоянного сбора flow logs включите экспорт через ресурс [`HubbleMonitoringConfig`](/modules/cni-cilium/cr.html#hubblemonitoringconfig). Конфигурация описана в [примерах модуля cni-cilium](/modules/cni-cilium/examples.html#hubblemonitoringconfig).

После включения экспорта `cilium-agent` пишет события в файл `/var/log/cilium/hubble/flow.log` на каждом узле. Для централизованного сбора используйте модуль [`log-shipper`](/modules/log-shipper/) с ресурсом `ClusterLoggingConfig` типа `File`, читающим этот файл.

{% alert level="warning" %}
Изменение `HubbleMonitoringConfig` приводит к перезапуску всех агентов Cilium в кластере.
{% endalert %}

## Диагностика FQDN-правил

Если правило с `toFQDNs` не пропускает трафик, проверьте кеш сопоставлений DNS-имя → IP-адрес, который ведёт `cilium-agent`:

```bash
d8 k -n d8-cni-cilium exec ds/agent -- cilium-dbg fqdn cache list
```

В выводе видны записи с источником, DNS-именем, разрешёнными IP-адресами и TTL. Если для нужного имени записи нет, под либо не делал DNS-запрос, либо DNS-запрос не разрешён политикой и не попал в инспекцию (`rules.dns`). Механика обновления кеша описана в разделе [DNS Policy и IP Discovery](cilium_networkpolicy.html#dns-policy-и-ip-discovery).

Дополнительно посмотрите вердикты политик для DNS-запросов:

```bash
d8 k -n d8-cni-cilium exec ds/agent -- hubble observe --type policy-verdict --port 53
```

## Типовые ошибки

### DNS не работает после default-deny egress

Default-политика deny-egress блокирует и DNS-запросы. Добавьте правило egress на сервис kube-dns в неймспейсе `kube-system` (порты UDP/53 и TCP/53). Подробности — в разделе [«Политики по умолчанию для неймспейса»](kubernetes_networkpolicy.html#политики-по-умолчанию-для-неймспейса).

### Перепутан AND и OR в селекторах

В одном элементе массива `from`/`to` два селектора — это пересечение, в двух отдельных элементах — объединение. Проверьте структуру в разделе [«Различие между объединением и пересечением селекторов»](kubernetes_networkpolicy.html#различие-между-объединением-и-пересечением-селекторов).

### Политика не действует на поды `hostNetwork`

Большинство движков, включая Cilium и kube-router, не различают такие поды и трафик узла. Для фильтрации используйте [host firewall на узлах](host_firewall.html).

### FQDN-правило не пропускает трафик

Cilium должен видеть DNS-запросы, чтобы поддерживать актуальный список IP-адресов. В одной политике с `toFQDNs` обязательно разрешите egress на kube-dns и включите DNS-инспекцию через `rules.dns`. Пример — в разделе [«FQDN-правила»](cilium_networkpolicy.html#fqdn-правила).

### Соединение разрывается после изменения политики

Поведение для уже установленных соединений не определено стандартом — некоторые движки разрывают такие соединения. Меняйте политики в окне обслуживания.

## Чек-лист «политика не применяется»

Если ресурс создан, но трафик не ведёт себя ожидаемо, последовательно проверьте:

1. Какой движок включён. Стандартный `NetworkPolicy` поддерживается обоими движками; `CiliumNetworkPolicy`, `CiliumClusterwideNetworkPolicy`, L7 и FQDN — только в кластерах с `cni-cilium`. Возможности движков перечислены в разделе [«Что доступно в зависимости от движка»](configuration.html#что-доступно-в-зависимости-от-движка).
1. Selector действительно выбирает поды: `d8 k get pods -n <namespace> -l <key>=<value>` должен вернуть ожидаемый список.
1. `policyTypes` указан корректно. Если перечислен только `Ingress`, egress не ограничен; если только `Egress`, ingress не ограничен.
1. AND vs OR в селекторах. Проверьте структуру массива — частая причина «слишком широкого» или «слишком узкого» правила.
1. Режим аудита. Если включён [`policyAuditMode`](/modules/cni-cilium/configuration.html#parameters-policyauditmode), политики не блокируют трафик. В `cilium-dbg endpoint list` это видно как `Disabled (Audit)`.
1. Eventual consistency. После создания политики Cilium и kube-router применяют её асинхронно. Подождите несколько секунд и повторите проверку.
1. Статус политики (только для `CiliumNetworkPolicy` и `CiliumClusterwideNetworkPolicy`). `d8 k get ciliumnetworkpolicy <name> -n <namespace> -o yaml` покажет в `status` ошибки парсинга или применения.
1. Конфликт с deny-правилом. Deny-правила Cilium имеют приоритет над любыми allow-правилами. Найдите политики с `ingressDeny` и `egressDeny`, выбирающие тот же эндпоинт.

## Дополнительная документация

- [HubbleMonitoringConfig — модуль cni-cilium](/modules/cni-cilium/cr.html#hubblemonitoringconfig)
- [Troubleshooting Policy — документация Cilium](https://docs.cilium.io/en/v1.17/security/policy/#troubleshooting)
- [Стандартный NetworkPolicy Kubernetes](kubernetes_networkpolicy.html)
- [CiliumNetworkPolicy и CiliumClusterwideNetworkPolicy](cilium_networkpolicy.html)
- [Host firewall на узлах](host_firewall.html)
