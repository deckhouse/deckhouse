---
title: "Модуль cni-cilium: расширенное использование"
description: "Объединение сетей подов кластеров, связанных пирингом: требования, включение, соседи, статус, отключение, диагностика."
---

## Пиринг кластеров

{% alert level="warning" %}
Экспериментальная функция, доступна только в редакции Ultimate/EE. Ресурсы ClusterMesh и ClusterNetworkPeer — версии `v1alpha1`.
{% endalert %}

Модуль объединяет сети подов кластеров, связанных пирингом модуля [`peering`](/modules/peering/): под обращается к поду другого кластера по IP-адресу, сервис может использовать бэкенды других кластеров (подробнее — [в разделе «Пиринг кластеров»](usage.html#пиринг-кластеров) использования). Через пиринг кластеры только обмениваются настройками объединения; трафик идёт напрямую между узлами.

Ограничения:

- только режим VXLAN без шифрования; трафик подов между кластерами не шифруется — объединяйте кластеры только через доверенные сети;
- соседи подключаются к control-plane-узлам напрямую, по адресам узлов; балансировщик и NAT между кластерами не поддерживаются;
- подсети подов кластеров не должны пересекаться;
- кластер и все его соседи — не больше 255 кластеров;
- объединение не транзитивно: если A объединён с B, а B — с C, то A и C не связаны;
- включение, выключение и смена ID кластера меняют идентичности безопасности всех подов: после них перезапустите все поды кластера — делайте это в окно обслуживания.

### Требования

| Требование | Как проверить |
|---|---|
| Модуль `peering` включён, `ClusterPeer` в фазе `Active` с обеих сторон | `d8 k get clusterpeers` |
| Cilium в режиме VXLAN, [`tunnelMode: VXLAN`](configuration.html#parameters-tunnelmode) задан явно в ModuleConfig | `d8 k get mc cni-cilium -o jsonpath='{.spec.settings.tunnelMode}'` выводит `VXLAN` |
| Шифрование выключено: [`encryption.mode`](configuration.html#parameters-encryption-mode) не задан или `Disabled` | `d8 k get mc cni-cilium -o jsonpath='{.spec.settings.encryption.mode}'` |
| Одинаковый порт VXLAN у всех кластеров | `d8 k -n d8-cni-cilium get cm cilium-config -o jsonpath='{.data.tunnel-port}'` |
| Подсеть подов не пересекается с подсетями подов соседей | `podSubnetCIDR` в `d8 k -n kube-system get secret d8-cluster-configuration -o jsonpath='{.data.cluster-configuration\.yaml}' \| base64 -d` |
| ID кластера уникален среди кластера и всех его соседей | Выдавайте ID из одного списка для всех кластеров, которые могут быть объединены |

Порт VXLAN задаёт модуль. Кластеры с разными портами объединить нельзя. Включение модуля `virtualization` может сменить порт.

Подсети сервисов могут пересекаться.

Сетевой доступ между кластерами, по адресам `InternalIP` узлов, без NAT:

| Откуда | Куда | Порт |
|---|---|---|
| Каждый узел | Каждый узел соседа | UDP, порт VXLAN |
| Control-plane-узлы | Control-plane-узлы соседа | TCP, `clusterMesh.hostPort.port` соседа (по умолчанию 4294) |

### Включение

Пример объединяет кластеры A и B. Пиринг называется `cluster-b` в кластере A и `cluster-a` в кластере B. Каждый шаг выполняется в обоих кластерах.

1. Свяжите кластеры пирингом по [руководству модуля `peering`](/modules/peering/admin_guide.html) до фазы `Active`. Возможности пока не открывайте.

1. Включите ClusterMesh в ModuleConfig `cni-cilium` (`d8 k edit mc cni-cilium`). В кластере B — `clusterID: 2`:

   ```yaml
   spec:
     settings:
       tunnelMode: VXLAN
       clusterMesh:
         enabled: true
         clusterID: 1
   ```

   Агенты Cilium перезапустятся. Затем перезапустите все поды кластера: они получат идентичности с ID кластера. Делайте это в окно обслуживания. Если Cilium работает без туннеля, переключите его на VXLAN отдельным окном обслуживания заранее.

   Дождитесь `READY: True` в `d8 k get clustermesh cluster` и `ACCEPTED: True` в `d8 k get peeringcapability network`.

1. Добавьте `network` в `spec.capabilities` объекта `ClusterPeer`, сохранив остальные возможности (`d8 k edit clusterpeer cluster-b`). Возможность работает, когда её перечислили обе стороны: проверьте `EFFECTIVE` в `d8 k get clusterpeer cluster-b`.

1. Создайте ClusterNetworkPeer с именем `ClusterPeer`. В кластере B — `cluster-a`:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterNetworkPeer
   metadata:
     name: cluster-b
   spec: {}
   ```

1. Дождитесь `PHASE: Configured` и `READY: True` в `d8 k get clusternetworkpeers`. Обычно это минута-две.

Если ModuleConfig отклонён, исправьте настройку из сообщения: задайте `clusterMesh.clusterID`, явный `tunnelMode: VXLAN` или выключите шифрование.

### Несколько соседей

С каждым соседом кластер объединяется отдельно: `ClusterPeer` с возможностью `network` и ClusterNetworkPeer с обеих сторон.

ID и подсеть подов соседа заняты, пока в кластере есть его ClusterNetworkPeer, даже если объединение сейчас не работает. Новый сосед с занятым ID или подсетью получает ошибку (`ClusterIDConflict`, `PodCIDROverlap`), уже объединённый сосед продолжает работать. Чтобы устранить конфликт, смените ID нового соседа или удалите ClusterNetworkPeer старого.

### Статус ClusterNetworkPeer

Фазы:

- `Pending` — сосед не дал согласия или не открыл `network`;
- `Negotiating` — обмен сертификатами;
- `Configured` — объединение настроено;
- `Error` — проверка не пройдена, причина — в условиях.

Чтобы вывести условия, выполните:

```shell
d8 k get clusternetworkpeer cluster-b \
  -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'
```

| Причина | Что сделать |
|---|---|
| `PeerNotActive` | Назовите ClusterNetworkPeer так же, как `ClusterPeer`, и проверьте пиринг |
| `CapabilityNotRequested` | Добавьте `network` в `spec.capabilities` объекта `ClusterPeer` |
| `CapabilityNotEffective` | Добавьте `network` на стороне соседа |
| `AwaitingPeer` | Создайте у соседа ClusterNetworkPeer с именем из сообщения |
| `PeerUnreachable`, `ClusterPeerUnreadable`, `PeerAccessNotReady` | Проверьте пиринг. С `... not found` — дождитесь, пока у соседа развернётся ClusterMesh |
| `TunnelModeNotVXLAN`, `VXLANPortMismatch` | Переведите соседа на VXLAN с тем же портом. Кластеры с разными портами объединить нельзя |
| `ClusterIDConflict` | Смените ID соседа или удалите ClusterNetworkPeer соседа из сообщения |
| `ClusterNameConflict` | Удалите лишний пиринг с тем же кластером. Склонированный вместе с UUID кластер объединить нельзя |
| `PodCIDROverlap` | Подсети подов (podCIDR) кластеров пересекаются. Такие кластеры объединить нельзя |
| `RemoteNotReady` | Сосед ещё не опубликовал своё состояние. Если не проходит — проверьте у соседа `d8 k get clustermesh cluster` и реплики `clustermesh-apiserver` |
| `AwaitingPeerSignature` | Подождите следующей проверки. Если не проходит — смотрите ClusterNetworkPeer соседа |
| `PeerCSRInvalid` | Пересоздайте ClusterNetworkPeer на стороне соседа |
| `LastKnownConfiguration` | Пиринг недоступен, объединение работает на последней конфигурации. Восстановите пиринг |

### Изменение настроек

- Чтобы сменить `clusterMesh.clusterID`, проверьте, что новый ID свободен среди кластера и его соседей. Меняйте его в окно обслуживания и затем перезапустите все поды кластера.
- Чтобы сменить `clusterMesh.hostPort.port`, сначала откройте новый порт в межсетевых экранах соседей.

### Отключение соседа

Чтобы отключить соседа, удалите его ClusterNetworkPeer на любой стороне:

```shell
d8 k delete clusternetworkpeer cluster-b
```

Объединение разрывается с обеих сторон, соединения между кластерами обрываются. Чтобы восстановить объединение, создайте ClusterNetworkPeer заново.

Чтобы занять ID или подсеть выведенного кластера другим кластером, удалите ClusterNetworkPeer выведенного кластера у всех его соседей.

### Отключение ClusterMesh

1. Удалите все ClusterNetworkPeer.
1. Задайте `clusterMesh.enabled: false` в ModuleConfig `cni-cilium` в окно обслуживания и затем перезапустите все поды кластера.
1. Уберите `network` из `spec.capabilities` объектов `ClusterPeer`.
