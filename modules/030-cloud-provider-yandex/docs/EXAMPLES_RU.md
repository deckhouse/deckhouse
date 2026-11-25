---
title: "Cloud provider — Yandex Cloud: примеры"
---

Ниже представлен пример конфигурации cloud-провайдера Yandex Cloud.

## Пример конфигурации модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    additionalExternalNetworkIDs:
    - enp6t4snovl2ko4p15em
```

## Управление зонами

Параметр [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) задаёт общий список зон кластера. Используйте этот раздел при установке кластера и при изменении списка зон в работающем кластере. Для работающего кластера сначала выполните проверки перед изменением списка зон.

В автоматическом режиме, если параметр [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) не задан, Deckhouse Platform использует зоны `ru-central1-a`, `ru-central1-b`, `ru-central1-d` и `ru-central1-e`. В непустом списке можно указать от одной до четырёх поддерживаемых зон. Чтобы использовать `ru-central1-f`, укажите её явно. Поведение при использовании [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) описано в разделе [«Использование существующих подсетей»](#использование-существующих-подсетей).

### Выбор зон для кластера

При автоматическом создании подсетей Deckhouse Platform делит [`nodeNetworkCIDR`](cluster_configuration.html#yandexclusterconfiguration-nodenetworkcidr) на четыре равные части. Первая часть диапазона соответствует зоне `ru-central1-a`, вторая часть соответствует зоне `ru-central1-b`, третья часть соответствует зоне `ru-central1-e`, а четвёртая часть соответствует зоне `ru-central1-d`.

Выберите вкладку, соответствующую списку зон.

{% tabs zone_selection %}

{% tab "Только зоны по умолчанию" %}

При автоматическом создании подсетей, если параметр [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) не задан или содержит только зоны по умолчанию, Deckhouse Platform создаёт или сохраняет подсети всех четырёх зон по умолчанию. Это происходит даже тогда, когда в списке указана только одна зона. Список зон в конфигурации группы узлов определяет, в каких из этих подсетей размещаются узлы.

Чтобы использовать зоны по умолчанию, не задавайте параметр [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones). Пустой список `[]` допускается схемой, но отличается от незаданного параметра при формировании списка зон для групп узлов.

{% endtab %}

{% tab "Зона ru-central1-f" %}

Зону `ru-central1-f` можно указать без зон по умолчанию. Например, задайте в ресурсе [YandexClusterConfiguration](cluster_configuration.html#yandexclusterconfiguration) следующий список:

```yaml
zones:
  - ru-central1-f
```

Если [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) не задан или пуст, Deckhouse Platform создаёт подсеть только в `ru-central1-f`. Для неё используется часть диапазона, освобождённая одной из зон по умолчанию. При списке только из `ru-central1-f` используется часть диапазона `ru-central1-a`. Пока `ru-central1-f` остаётся в списке, Deckhouse Platform сохраняет CIDR уже созданной подсети. Если удалить зону из списка, подсеть будет удалена вместе с ней.

При замене зоны по умолчанию на `ru-central1-f` новая подсеть получает часть диапазона исключённой зоны. Например, при замене `ru-central1-e` на `ru-central1-f` используется третья часть диапазона. Подсеть `ru-central1-e` удаляется после того, как из неё выведены узлы и другие ресурсы.

Если в [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) или [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) задан список зон, добавьте в него `ru-central1-f` для групп, которым нужны узлы в этой зоне. Для группы [NodeGroup](/modules/node-manager/cr.html#nodegroup) используйте параметр [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones).

{% endtab %}

{% endtabs %}

### Использование существующих подсетей

Если вы заранее создали подсети в Yandex Cloud и хотите использовать их для узлов кластера, укажите соответствие зон и идентификаторов подсетей в [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap). Deckhouse Platform использует эти подсети, но не управляет их жизненным циклом. Для одной зоны можно указать один идентификатор подсети.

Параметр [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) может содержать больше четырёх зон. Ограничение в четыре зоны относится только к явно заданному списку [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones). Чтобы однозначно выбрать зоны для размещения, задайте [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) и включите в него зоны, которые есть в параметре [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap).

Пример конфигурации с подсетями пяти зон и размещением узлов в двух из них:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 10.241.32.0/20
sshPublicKey: ssh-rsa AAAAB3Nza...
zones:
  - ru-central1-a
  - ru-central1-f
existingZoneToSubnetIDMap:
  ru-central1-a: <RU_CENTRAL1_A_SUBNET_ID>
  ru-central1-b: <RU_CENTRAL1_B_SUBNET_ID>
  ru-central1-d: <RU_CENTRAL1_D_SUBNET_ID>
  ru-central1-e: <RU_CENTRAL1_E_SUBNET_ID>
  ru-central1-f: <RU_CENTRAL1_F_SUBNET_ID>
```

В этом примере подсети `ru-central1-b`, `ru-central1-d` и `ru-central1-e` остаются в параметре [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) и в Yandex Cloud, но не входят в список зон для размещения узлов. Чтобы использовать одну из них, добавьте её в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) и в списки зон нужных групп узлов.

Если [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) не задан, Deckhouse Platform использует подсети из [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap). Для CloudEphemeral список зон по умолчанию формируется из данных обнаружения. Если параметр содержит зоны по умолчанию, в этот список входят только они. Если параметр содержит только зоны вне набора по умолчанию, в список входят все указанные в нём зоны. Для CloudPermanent без собственного списка зон используются все подсети, указанные в параметре. Чтобы однозначно ограничить размещение, задайте [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) явно и при необходимости укажите списки зон групп узлов.

Чтобы добавить новую зону, заранее создайте в ней подсеть и добавьте её идентификатор в [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap). Затем включите зону в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) и в списки зон групп узлов, которые должны размещать в ней узлы. При изменении параметра сохраняйте записи остальных зон, которые продолжаете использовать.

{% alert level="warning" %}
Если удалить все записи из [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), Deckhouse Platform перейдёт к автоматическому созданию подсетей. Перед удалением последней записи убедитесь, что это соответствует плану изменения инфраструктуры.
{% endalert %}

### Изменение состава зон в работающем кластере

Изменение списка зон не переносит узлы, диски и другие ресурсы между зонами автоматически. Сначала измените списки зон групп узлов и перенесите рабочие нагрузки, затем измените общий список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones). [Диск Yandex Cloud, привязанный к зоне](https://yandex.cloud/ru/docs/compute/concepts/disk), можно подключить только к узлу в той же зоне. Перед изменением перенесите данные по процедуре для соответствующей рабочей нагрузки. Общая информация о дисках и PersistentVolume приведена в [разделе о хранилище Yandex Cloud](/products/kubernetes-platform/documentation/v1/admin/integrations/public/yandex/storage.html).

{% alert level="warning" %}
Подсеть нельзя удалить, пока её используют узлы или другие ресурсы Yandex Cloud. Перед изменением выведите узлы из исключаемой зоны и отключите от удаляемой подсети остальные ресурсы. Порядок удаления подсети описан в [документации Yandex Cloud](https://yandex.cloud/ru/docs/vpc/operations/subnet-delete).
{% endalert %}

Если в схеме [`WithNATInstance`](layouts.html#withnatinstance) не заданы [`withNATInstance.internalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-internalsubnetid) и [`withNATInstance.externalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-externalsubnetid), зона NAT-инстанса определяется по списку [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

| Список зон | Размещение NAT-инстанса |
| --- | --- |
| Содержит только зоны по умолчанию | NAT-инстанс остаётся в `ru-central1-a` |
| Содержит динамическую зону и `ru-central1-a` | NAT-инстанс остаётся в `ru-central1-a` |
| Содержит зону вне набора по умолчанию, а `ru-central1-a` исключена | NAT-инстанс пересоздаётся в первой по алфавиту зоне из списка [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) |

{% alert level="warning" %}
Если используется [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), но в нём нет подсети `ru-central1-a`, задайте [`withNATInstance.internalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-internalsubnetid) или [`withNATInstance.externalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-externalsubnetid) явно. Иначе NAT-инстанс не сможет определить подсеть в зоне `ru-central1-a`.
{% endalert %}

{% alert level="warning" %}
Исходящий трафик кластера прерывается на время пересоздания NAT-инстанса.
{% endalert %}

{% alert level="warning" %}
По информации Yandex Cloud, с 16 декабря 2026 года операции создания и изменения виртуальных машин в зоне `ru-central1-b` будут недоступны. До этой даты выведите узлы из зоны по одному из примеров ниже. Сохранение подсети не возвращает возможность создавать или изменять виртуальные машины в зоне.
{% endalert %}

Перед изменением списка зон выполните следующие проверки:

1. Убедитесь, что установленная версия Deckhouse Platform поддерживает выбранные зоны. Доступные значения [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) приведены в [справочнике YandexClusterConfiguration](cluster_configuration.html#yandexclusterconfiguration).
1. Проверьте состояние кластера и создайте резервную копию etcd. Порядок действий описан в [разделе о резервном копировании etcd](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html#резервное-копирование-etcd).
1. Проверьте квоту вычислительных ресурсов в зонах из временного и итогового списков групп узлов. При замене единственного master-узла предусмотрите ресурсы для временного увеличения их количества до трёх. Порядок изменения master-узлов описан в [разделе об управлении master-узлами](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#типовые-сценарии-масштабирования).
1. Для групп CloudPermanent проверьте параметры [`externalSubnetIDs`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalsubnetids) и [`externalIPAddresses`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalipaddresses). Узлы распределяются по зонам в алфавитном порядке, список зон повторяется для следующих узлов. После изменения зон убедитесь, что внешняя подсеть и внешний IP-адрес каждого узла соответствуют его зоне.
1. Для каждой группы CloudEphemeral проверьте механизм управления узлами:

   ```shell
   d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
   ```

   Процедура ниже предназначена для групп со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) и для CloudPermanent. В группе со [`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) старые зональные MachineDeployment не удаляются автоматически. Сначала выполните [процедуру миграции группы на Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi), а общий список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) изменяйте после удаления старой группы.
1. Найдите узлы в исключаемой зоне. Например, для `ru-central1-e` выполните:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

1. Проверьте, что рабочие нагрузки не используют диски, привязанные к исключаемой зоне. Перенесите данные таких дисков по процедуре для соответствующей рабочей нагрузки.
1. Проверьте, что к подсети исключаемой зоны не подключены другие ресурсы Yandex Cloud.

#### Порядок замены зоны

Ниже приведён пример замены `ru-central1-e` на `ru-central1-f` при автоматическом создании подсетей. В YAML-фрагментах показаны только изменяемые поля. Сохраните остальные параметры конфигурации.

{% alert level="warning" %}
В следующей процедуре изменение списка зон в [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) описано для групп CloudEphemeral со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine). Для групп со [`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) этот порядок не применяйте. Старые зональные MachineDeployment не удаляются автоматически, поэтому сначала выполните миграцию группы по ссылке из шага проверки, а затем измените общий список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones).
{% endalert %}

1. Задайте временные списки зон для всех групп узлов, которые используют `ru-central1-e`, и исключите эту зону из списков.
   Для CloudEphemeral со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) соответствующих NodeGroup. Для CloudPermanent измените [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) с помощью `d8 system edit provider-cluster-configuration`. Если список зон группы не задан, укажите его явно, чтобы вывести узлы из зоны до изменения общего списка.

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-b
       - ru-central1-d
   ```

1. Примените изменения групп узлов. Для CloudPermanent используйте `dhctl converge`. Для CloudEphemeral со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) дождитесь удаления соответствующего MachineDeployment и узлов. Если изменяете master-узлы, соблюдайте [порядок их замены](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#типовые-сценарии-масштабирования).
1. Убедитесь, что в `ru-central1-e` больше нет узлов:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   Команда не должна вывести имена узлов.
1. Выполните `d8 system edit provider-cluster-configuration` и замените в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) `ru-central1-e` на `ru-central1-f`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-d
     - ru-central1-f
   ```

1. Примените изменение с помощью `dhctl converge`. Deckhouse Platform удалит подсеть `ru-central1-e` и создаст подсеть `ru-central1-f` с тем же CIDR-слотом.
1. Добавьте `ru-central1-f` в списки зон групп узлов, которые должны размещать в ней узлы. Примените изменения групп узлов тем же способом, который указан в шаге 2.
1. Проверьте, что узлы и подсети соответствуют итоговой конфигурации:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

Если при `dhctl converge` появляется ошибка о пересечении CIDR, дождитесь завершения удаления старой подсети и повторите применение. Если ошибка сохраняется, удалите `ru-central1-f` из [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones), примените изменение с помощью `dhctl converge`, а затем снова задайте итоговый список с `ru-central1-f` и примените его.

Если кластер уже использует [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), Deckhouse Platform не создаёт и не удаляет подсети. В этом случае заранее создайте подсеть `ru-central1-f`, добавьте её идентификатор в параметр и измените [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones). Удаляйте запись `ru-central1-e` только после того, как перестанете использовать эту подсеть. Не переводите кластер из автоматического режима в режим существующих подсетей без отдельной процедуры.

#### Вывод из эксплуатации зоны ru-central1-b

Ниже приведены примеры для кластера с четырьмя зонами по умолчанию и автоматическим созданием подсетей. Если состав зон в вашем кластере отличается, сохраните нужные зоны в итоговом списке. Для NodeGroup изменяйте [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones). Для CloudPermanent изменяйте [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) в YandexClusterConfiguration.

Если группа CloudEphemeral использует CAPI, не исключайте `ru-central1-b` простым изменением списка зон. Старые зональные MachineDeployment не удаляются автоматически. Для такой группы сначала используйте [процедуру миграции на Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi), а затем изменяйте общий список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) после удаления старой группы в предусмотренной для кластера последовательности. Для группы со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) измените список зон по шагам выбранного примера.

Выберите один из примеров. Перед изменением общего списка зон выведите узлы из `ru-central1-b` способом, соответствующим механизму управления группой.

{% tabs zones_b_migration %}

{% tab "Сохранить подсеть ru-central1-b" %}

Используйте этот пример, если подсеть `ru-central1-b` должна остаться в Yandex Cloud, а узлы в этой зоне больше не нужны.

1. Задайте для групп узлов временные списки зон без `ru-central1-b` и примените изменения. Для CloudPermanent используйте `dhctl converge`. Для CloudEphemeral со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) дождитесь удаления соответствующего MachineDeployment и узлов.
1. Убедитесь, что в `ru-central1-b` больше нет узлов:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

1. Выполните `d8 system edit provider-cluster-configuration` и укажите в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) оставшиеся зоны. Например:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
   ```

1. Примените изменение с помощью `dhctl converge`. Поскольку в списке остаются только зоны по умолчанию, Deckhouse Platform сохраняет подсети всех четырёх зон, включая `ru-central1-b`.
1. Если кластер уже использует [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), сохраните запись `ru-central1-b` в параметре. Deckhouse Platform продолжит использовать указанные подсети и не будет управлять их удалением.

{% endtab %}

{% tab "Заменить ru-central1-b на ru-central1-f" %}

Используйте этот пример, если в итоговом списке нужна зона `ru-central1-f`. Пример относится к автоматическому созданию подсетей.

1. Задайте для групп узлов временные списки зон без `ru-central1-b` и примените изменения. Для CloudPermanent используйте `dhctl converge`. Для CloudEphemeral со [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) дождитесь удаления соответствующего MachineDeployment и узлов.
1. Убедитесь, что в `ru-central1-b` больше нет узлов и других ресурсов Yandex Cloud.
1. Выполните `d8 system edit provider-cluster-configuration` и замените в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) `ru-central1-b` на `ru-central1-f`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
     - ru-central1-f
   ```

1. Примените изменение с помощью `dhctl converge`. Deckhouse Platform удалит подсеть `ru-central1-b` и создаст подсеть `ru-central1-f` в освобождённом CIDR-слоте.
1. Добавьте `ru-central1-f` в списки зон нужных групп узлов и примените изменения групп узлов.
1. Проверьте итоговую конфигурацию:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   В списке узлов не должно быть узлов из `ru-central1-b`. В Yandex Cloud должна быть подсеть `ru-central1-f`.

{% endtab %}

{% endtabs %}

Если удаление узлов не завершается, проверьте состояние соответствующей NodeGroup и сообщения об ошибках. Для CloudPermanent проверьте вывод `dhctl converge`. Для CloudEphemeral со [`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) вернитесь к [процедуре миграции на Cluster API (CAPI)](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi). Изменяйте общий список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) только после удаления узлов.

Не удаляйте управляемые виртуальные машины и объекты Machine вручную. Это может нарушить процесс изменения групп узлов.

## Пример custom resource `YandexInstanceClass`

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

## LoadBalancer

### Аннотации объекта Service

В кластере заданы значения по умолчанию для размещения ресурсов балансировщиков (сеть для целевой группы и подсеть для Listener). Эти значения выставляются автоматически во время развёртывания кластера и могут быть переопределены аннотациями на уровне конкретного Service.

> Поведение по умолчанию (внешний или внутренний LB) зависит от конфигурации кластера. Для явного выбора типа используйте аннотацию `yandex.cpi.flant.com/loadbalancer-external`.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — указывает NetworkID, в котором будет создана целевая группа для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-subnet-id` — задаёт SubnetID для Listener’ов создаваемого LB для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-address-ipv4` — задаёт предопределённый IPv4-адрес для Listener’ов (поддерживаются и внутренние, и внешние LB).
1. `yandex.cpi.flant.com/loadbalancer-external` — включает создание внешнего (external) LB для данного Service (используйте, если нужно явно создать внешний балансировщик). Переопределяет поведение по умолчанию.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени целевой группы, которую использует LoadBalancer. Аннотация на Service определяет целевую группу, но не выбирает узлы. Чтобы включить узлы в целевую группу, задайте аннотацию с тем же значением в параметре [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations) ресурса NodeGroup. Yandex Cloud Controller Manager включает в целевую группу все подходящие узлы с таким значением аннотации независимо от их NodeGroup. Имя целевой группы формируется как `<ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>`. Подробнее — в разделе [«Использование отдельной целевой группы для NodeGroup»](faq.html#использование-отдельной-целевой-группы-для-nodegroup).

Если для control plane или master-узлов создаются отдельные целевые группы, добавьте на master-узлы лейбл `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые целевые группы для балансировщиков.

Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте целевую группу с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

### Проверки состояния целевой группы

Параметры healthcheck’ов (для создаваемых целевых групп балансировщика):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).
