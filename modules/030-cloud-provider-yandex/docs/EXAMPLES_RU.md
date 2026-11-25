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
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: b1g0ldjdpa8rtm2ru3s4
        folderID: b1gq1vn5jqk0tmh4u7bp
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: ssh-rsa AAAAB3Nza...
        zones:
          - ru-central1-a
          - ru-central1-b
        labels:
          billing: prod
        externalSubnetIDs:
          worker:
            - e2lu8r1tbbtryhdpa9ro
        externalIPAddresses:
          worker:
            - Auto
    storage:
      parameters:
        excludedStorageClasses:
          - network-hdd
        provisionedStorageClasses:
          - name: network-ssd-64k
            type: network-ssd
            blockSize: 64Ki
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4snovl2ko4p15em
        additionalInternalNetworkIDs:
          - enpmhcdmtvs6ptdq3mkt
```

Ключ сервисного аккаунта в этом ресурсе не хранится, он задаётся [отдельным секретом](environment.html#секрет-с-учётными-данными).

## Управление зонами

Параметр [`zones`](configuration.html#parameters-nodes-parameters-zones) задаёт общий список зон кластера. Если [`zones`](configuration.html#parameters-nodes-parameters-zones) не задан или пуст, при автоматическом создании подсетей Deckhouse Platform использует зоны `ru-central1-a`, `ru-central1-b`, `ru-central1-d` и `ru-central1-e`. В явно заданном непустом списке можно указать от одной до четырёх поддерживаемых зон.

В текущем наборе поддерживаемых зон `ru-central1-f` не входит в список по умолчанию. Чтобы использовать её в списке из четырёх зон, замените одну из зон по умолчанию. Пример замены `ru-central1-e` на `ru-central1-f` приведён в разделе [«Порядок замены зоны»](#порядок-замены-зоны). Если нужно использовать меньше четырёх зон, укажите только зоны, в которых должны размещаться узлы.

### Выбор зон для кластера

Перед установкой кластера или изменением списка зон в работающем кластере проверьте, какие подсети Deckhouse Platform создаст, сохранит или удалит.
Для изменения зон работающего кластера используйте [пошаговую инструкцию](#изменение-состава-зон-в-работающем-кластере).

Для автоматического создания подсетей Deckhouse Platform делит [`nodeNetworkCIDR`](configuration.html#parameters-nodes-parameters-nodenetworkcidr) на четыре равные части, CIDR-слоты. Каждой зоне по умолчанию соответствует свой CIDR-слот. Если параметр [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) не задан или пуст, набор создаваемых подсетей зависит от значения [`zones`](configuration.html#parameters-nodes-parameters-zones). Выберите подходящую вкладку. Если [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) содержит записи, перейдите [к разделу «Использование существующих подсетей»](#использование-существующих-подсетей).

{% tabs zone_selection %}

{% tab "Только зоны по умолчанию" %}

Если [`zones`](configuration.html#parameters-nodes-parameters-zones) не задан, пуст или содержит только зоны по умолчанию, Deckhouse Platform создаёт или сохраняет подсети всех четырёх зон по умолчанию, даже если в списке указана только одна зона.
Размещение узлов зависит от настроек провайдера и групп узлов.

{% endtab %}

{% tab "Не только зоны по умолчанию" %}

Если в [`zones`](configuration.html#parameters-nodes-parameters-zones) есть зона вне набора по умолчанию, например `ru-central1-f`, Deckhouse Platform создаёт подсети выбранных зон и удаляет подсети исключённых зон по умолчанию. Для новой подсети Deckhouse Platform использует CIDR-слот одной из невыбранных зон по умолчанию.

Можно указать только `ru-central1-f`, без зон по умолчанию. Например, фрагмент ModuleConfig для такого списка:

```yaml
spec:
  settings:
    nodes:
      parameters:
        zones:
          - ru-central1-f
```

В этом случае Deckhouse Platform создаёт или сохраняет подсеть узлов только в `ru-central1-f`. Для новой подсети используется первый CIDR-слот, закреплённый за `ru-central1-a`, то есть четверть диапазона [`nodeNetworkCIDR`](configuration.html#parameters-nodes-parameters-nodenetworkcidr). Если подсеть `ru-central1-f` уже существует, её CIDR сохраняется.

Чтобы размещать все узлы только в `ru-central1-f`, укажите эту зону в списках зон групп узлов, если они заданы явно. В NodeGroup измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones), а для постоянных групп в YandexClusterConfiguration измените [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).

{% alert level="warning" %}
Перед изменением списка зон в работающем кластере отключите от всех исключаемых подсетей использующие их ресурсы. Иначе Deckhouse Platform не сможет удалить такие подсети.

Чтобы оставить в работающем кластере только `ru-central1-f`, сначала [добавьте эту зону](#порядок-замены-зоны) и перенесите в неё узлы и данные. Исключайте остальные зоны после завершения переноса.
{% endalert %}

{% endtab %}

{% endtabs %}

### Использование существующих подсетей

Если вы заранее создали подсети в Yandex Cloud и хотите использовать их для узлов кластера, укажите соответствие зон и идентификаторов подсетей в [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Deckhouse Platform использует эти подсети, но не управляет ими. Для каждой зоны можно указать только один идентификатор подсети.

Если [`zones`](configuration.html#parameters-nodes-parameters-zones) не задан или пуст, общим списком зон кластера служат зоны из [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap), включая зоны вне набора по умолчанию. Чтобы ограничить этот список, задайте [`zones`](configuration.html#parameters-nodes-parameters-zones) явно и выберите зоны из [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

В следующем фрагменте ModuleConfig параметр [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) содержит подсети пяти зон, а [`zones`](configuration.html#parameters-nodes-parameters-zones) ограничивает общий список зонами `ru-central1-a` и `ru-central1-f`:

```yaml
spec:
  settings:
    nodes:
      parameters:
        existingZoneToSubnetIDMap:
          ru-central1-a: <RU_CENTRAL1_A_SUBNET_ID>
          ru-central1-b: <RU_CENTRAL1_B_SUBNET_ID>
          ru-central1-d: <RU_CENTRAL1_D_SUBNET_ID>
          ru-central1-e: <RU_CENTRAL1_E_SUBNET_ID>
          ru-central1-f: <RU_CENTRAL1_F_SUBNET_ID>
        zones:
          - ru-central1-a
          - ru-central1-f
```

Замените `<RU_CENTRAL1_A_SUBNET_ID>`, `<RU_CENTRAL1_B_SUBNET_ID>`, `<RU_CENTRAL1_D_SUBNET_ID>`, `<RU_CENTRAL1_E_SUBNET_ID>` и `<RU_CENTRAL1_F_SUBNET_ID>` идентификаторами подсетей соответствующих зон.

Подсети `ru-central1-b`, `ru-central1-d` и `ru-central1-e` остаются в [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) и сохраняются в Yandex Cloud. В приведённой конфигурации их зоны не входят в общий список зон для размещения узлов.

Чтобы размещать новые узлы в одной из этих подсетей, включите её зону в [`zones`](configuration.html#parameters-nodes-parameters-zones), сохранив в списке не более четырёх зон. Если в нужных группах узлов явно заданы списки зон, добавьте в них ту же зону.

Если [`zones`](configuration.html#parameters-nodes-parameters-zones) не задан или пуст, общий список включает все пять зон из [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

Чтобы добавить новую зону, заранее создайте в ней подсеть и добавьте её идентификатор в [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Сохраните записи остальных зон. Если список [`zones`](configuration.html#parameters-nodes-parameters-zones) непустой, включите в него новую зону.

Затем добавьте новую зону в настройки групп узлов, которые должны создавать узлы в этой зоне. Для NodeGroup измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones). Для постоянных групп узлов в YandexClusterConfiguration измените [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) или [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).

{% alert level="warning" %}
Сохраняйте хотя бы одну запись в [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Если удалить параметр или все его записи, Deckhouse Platform перейдёт к автоматическому созданию подсетей.
{% endalert %}

Ограничение в четыре зоны относится к явному списку [`zones`](configuration.html#parameters-nodes-parameters-zones). Оно не ограничивает количество зон в [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

### Изменение состава зон в работающем кластере

Перед изменением зон ознакомьтесь с последствиями и выполните проверки ниже. Затем перейдите [к порядку замены зоны](#порядок-замены-зоны).

Deckhouse Platform не переносит ресурсы между зонами. При изменении зон рабочей NodeGroup Deckhouse Platform удаляет узлы из исключённых зон. Количество узлов в оставшихся зонах определяется настройками группы, а для CloudEphemeral зависит и от автомасштабирования. Зональный том использует диск Yandex Cloud, который можно [подключить только к узлу в той же зоне](https://yandex.cloud/ru/docs/compute/concepts/disk). До изменения зон перенесите рабочие нагрузки и данные томов по процедуре для соответствующего приложения. Общая информация о дисках и PersistentVolume приведена в [разделе о хранилище Yandex Cloud](/products/kubernetes-platform/documentation/v1/admin/integrations/public/yandex/load-balancing-and-storage.html).

{% alert level="warning" %}
Подсеть нельзя удалить, пока её используют другие ресурсы Yandex Cloud. Перед применением изменения выведите узлы из исключаемой зоны, перенесите данные томов и отключите остальные ресурсы от удаляемых подсетей. Проверки описаны в [инструкции Yandex Cloud по удалению подсети](https://yandex.cloud/ru/docs/vpc/operations/subnet-delete).
{% endalert %}

{% alert level="warning" %}
В схеме [`WithNATInstance`](layouts.html#withnatinstance) проверьте зону NAT-инстанса до удаления подсетей. Зона подсети из [`withNATInstance.internalSubnetID`](configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid) определяет зону NAT-инстанса. Если этот параметр не задан, используется зона подсети из [`withNATInstance.externalSubnetID`](configuration.html#parameters-nodes-parameters-withnatinstance-externalsubnetid). Если оба идентификатора не заданы, при автоматическом создании подсетей с `ru-central1-f` используется первая по алфавиту зона из [`zones`](configuration.html#parameters-nodes-parameters-zones), а в остальных случаях используется `ru-central1-a`.

Если NAT-инстанс находится в исключаемой зоне, перенесите его до удаления подсетей. Чтобы использовать существующую подсеть в целевой зоне, укажите её идентификатор в [`withNATInstance.internalSubnetID`](configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid) и удалите [`withNATInstance.internalSubnetCIDR`](configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetcidr), если он задан. Проверьте остальные [настройки NAT-инстанса](configuration.html#parameters-nodes-parameters-withnatinstance). Внешняя подсеть должна находиться в той же зоне, что и NAT-инстанс. Проверьте соответствие явно заданных [`natInstanceInternalAddress`](configuration.html#parameters-nodes-parameters-withnatinstance-natinstanceinternaladdress) и [`natInstanceExternalAddress`](configuration.html#parameters-nodes-parameters-withnatinstance-natinstanceexternaladdress) новым подсетям и зоне. Примените изменения с помощью `dhctl converge`.

Если внутренний IP-адрес NAT-инстанса изменился, обновите адрес следующего узла (next hop) для маршрута `0.0.0.0/0` в [таблице маршрутизации](https://yandex.cloud/ru/docs/vpc/cli-ref/route-table/update), сохранив остальные маршруты. При переносе NAT-инстанса исходящий трафик прерывается до завершения переноса и обновления маршрута.
{% endalert %}

Если при `dhctl converge` создание подсети завершается ошибкой из-за пересечения CIDR с удаляемой подсетью, проверьте завершение удаления и повторите `dhctl converge`. Это возможно при любой замене подсети, включая восстановление CIDR-слота и замену `ru-central1-b` на `ru-central1-f`.

Перед изменением списка зон выполните следующие проверки:

1. Убедитесь, что установленная версия Deckhouse Platform поддерживает выбранные зоны. Доступные значения параметра [`zones`](configuration.html#parameters-nodes-parameters-zones) приведены в [настройках модуля `cloud-provider-yandex`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones).
1. Проверьте состояние кластера и создайте резервную копию etcd. Порядок действий описан в [разделе о резервном копировании etcd](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html#резервное-копирование-etcd).
1. Проверьте [свободные квоты Yandex Cloud](https://yandex.cloud/ru/docs/compute/concepts/limits) и доступность ресурсов для создания узлов во всех зонах из временных и итоговых списков зон групп узлов, включая уже используемые зоны.
   В примере замены `ru-central1-e` на `ru-central1-f` сначала потребуются ресурсы в `ru-central1-a`, `ru-central1-b` и `ru-central1-d`, затем в новой зоне `ru-central1-f`.
   При замене единственного master-узла `dhctl converge` временно увеличивает число master-узлов до трёх. Предусмотрите ресурсы для двух дополнительных master-узлов в зонах из нового списка группы `master`.
1. Найдите узлы в исключаемой зоне. Например, для `ru-central1-e` выполните:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   Пример вывода:

   ```console
   node/cluster-worker-e-0
   node/cluster-worker-e-1
   ```

1. Проверьте ограничения размещения PersistentVolume:

   ```shell
   d8 k get pv -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.nodeAffinity}{"\n"}{end}'
   ```

   В выводе найдите имя исключаемой зоны.
1. Перенесите данные томов, доступных только в исключаемой зоне, средствами соответствующего приложения.
   Общая информация о дисках и PersistentVolume приведена [в разделе о хранилище Yandex Cloud](/products/kubernetes-platform/documentation/v1/admin/integrations/public/yandex/load-balancing-and-storage.html).
1. Определите временный и итоговый списки зон каждой группы узлов. Используйте зоны из общего списка [`zones`](configuration.html#parameters-nodes-parameters-zones), в которых узлы должны размещаться на соответствующем этапе.
   Для NodeGroup типа `CloudEphemeral` указывайте список в [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) при любом способе конфигурации провайдера.
   Для групп типа `CloudPermanent` при конфигурации провайдера через ModuleConfig используйте тот же параметр NodeGroup. При устаревшем способе конфигурации через YandexClusterConfiguration изменяйте [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).
   На первом этапе исключите выводимую зону из списков использующих её групп. Новую зону добавляйте в группы после создания её подсети. Пошаговые примеры приведены ниже.
1. Для групп узлов типа `CloudPermanent` проверьте [`externalSubnetIDs`](configuration.html#parameters-nodes-parameters-externalsubnetids) и [`externalIPAddresses`](configuration.html#parameters-nodes-parameters-externalipaddresses). Deckhouse Platform распределяет эти узлы по зонам в алфавитном порядке, повторяя список зон для следующих узлов. После изменения зон убедитесь, что внешняя подсеть и внешний IP-адрес каждого узла соответствуют его зоне.

#### Порядок замены зоны

Сначала измените списки зон групп узлов и дождитесь удаления узлов из исключаемой зоны. Затем измените общий список зон в конфигурации провайдера.
Ниже приведён пример замены `ru-central1-e` на `ru-central1-f` с автоматическим созданием подсетей. Для другого набора зон измените значения в примерах.
В YAML-фрагментах показаны только изменяемые поля. Сохраните остальные параметры существующих ресурсов.
Выберите вкладку, соответствующую способу конфигурации кластера.

{% tabs zones_configuration %}

{% tab "ModuleConfig" %}

Чтобы заменить зону в кластере с конфигурацией через ModuleConfig, выполните следующие шаги:

1. Измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в каждой рабочей [NodeGroup](/modules/node-manager/cr.html#nodegroup), которая использует `ru-central1-e`.
   Временно оставьте зоны, которые доступны сейчас и останутся в итоговом списке:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   ```

1. Исключите `ru-central1-e` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в [NodeGroup](/modules/node-manager/cr.html#nodegroup) `master`.

1. Дождитесь завершения изменения групп узлов.
   Для `CloudEphemeral` модуль [`node-manager`](/modules/node-manager/) применяет изменение автоматически.
   Для групп типа `CloudPermanent`, включая `master`, примените изменения с помощью `dhctl converge`.
   Команда последовательно пересоздаёт master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений.
   После каждой замены master-узла выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Убедитесь, что в `ru-central1-e` больше нет узлов кластера:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   Команда не должна вывести имена узлов.
   Отдельно проверьте, что удаляемая подсеть больше не используется другими ресурсами Yandex Cloud.

1. Откройте ModuleConfig модуля командой `d8 k edit mc cloud-provider-yandex` и измените [`zones`](configuration.html#parameters-nodes-parameters-zones), заменив `ru-central1-e` на `ru-central1-f`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-d
             - ru-central1-f
   ```

1. Примените изменения с помощью `dhctl converge`.
   При автоматическом создании подсетей `ru-central1-f` получит CIDR-слот `ru-central1-e`.
   Если CIDR-слот уже существующей подсети `ru-central1-f` назначен зоне по умолчанию, выполните процедуру освобождения CIDR-слота ниже.

1. Добавьте `ru-central1-f` в [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) групп, которые должны использовать новую зону:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
         - ru-central1-f
   ```

   Для CloudPermanent снова примените изменения с помощью `dhctl converge`.
   Для CloudEphemeral дождитесь применения изменений модулем `node-manager`.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

Чтобы заменить зону в кластере с устаревшим способом конфигурации через YandexClusterConfiguration, выполните следующие шаги:

1. Измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) рабочих [NodeGroup](/modules/node-manager/cr.html#nodegroup) типа CloudEphemeral.
   Временно исключите `ru-central1-e`:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   ```

   Дождитесь применения изменений модулем [`node-manager`](/modules/node-manager/).

1. Откройте конфигурацию провайдера командой `d8 system edit provider-cluster-configuration`.
   В соответствующих элементах [`nodeGroups[]`](cluster_configuration.html#yandexclusterconfiguration-nodegroups) измените [`zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones).
   Например, для группы постоянных узлов `worker` укажите:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   ```

1. Исключите `ru-central1-e` из [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones), если в этой зоне находятся master-узлы:

   ```yaml
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-b
       - ru-central1-d
   ```

1. Дождитесь завершения изменения групп узлов.
   Для `CloudEphemeral` модуль [`node-manager`](/modules/node-manager/) применяет изменение автоматически.
   Для групп типа `CloudPermanent`, включая `master`, примените изменения с помощью `dhctl converge`.
   Команда последовательно пересоздаёт master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений.
   После каждой замены master-узла выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Убедитесь, что в `ru-central1-e` больше нет узлов кластера:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   Команда не должна вывести имена узлов.
   Отдельно проверьте, что удаляемая подсеть больше не используется другими ресурсами Yandex Cloud.

1. Снова выполните `d8 system edit provider-cluster-configuration` и измените глобальный список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-d
     - ru-central1-f
   ```

1. Примените изменения с помощью `dhctl converge`.
   При автоматическом создании подсетей `ru-central1-f` получит CIDR-слот `ru-central1-e`.
   Если CIDR-слот уже существующей подсети `ru-central1-f` назначен зоне по умолчанию, выполните процедуру освобождения CIDR-слота ниже.

1. Добавьте `ru-central1-f` в списки зон групп, которые должны использовать новую зону.
   Для CloudEphemeral измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в NodeGroup.
   Для постоянных групп измените [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) или [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) и снова примените изменения с помощью `dhctl converge`.
   В этом примере итоговый список групп содержит `ru-central1-a`, `ru-central1-b`, `ru-central1-d` и `ru-central1-f`.

Если кластер ещё использует YandexClusterConfiguration, ознакомьтесь с [переходом провайдера на ModuleConfig](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).

{% endtab %}

{% endtabs %}

Чтобы вернуть в список зону по умолчанию, чей CIDR-слот занят подсетью `ru-central1-f`, пересоздайте эту подсеть по инструкции ниже.
Выполните эту процедуру, если подсети создаёт Deckhouse Platform и вы хотите сохранить `ru-central1-f` в итоговом списке.
Например, при переходе со списка `[ru-central1-a, ru-central1-b, ru-central1-d, ru-central1-f]` на `[ru-central1-a, ru-central1-b, ru-central1-e, ru-central1-f]` сначала освободите CIDR-слот `ru-central1-e`.
Выберите вкладку, соответствующую способу конфигурации кластера.

{% tabs zones_cidr_restore %}

{% tab "ModuleConfig" %}

Чтобы освободить CIDR-слот и пересоздать подсеть `ru-central1-f`, выполните следующие шаги:

1. Исключите `ru-central1-f` и `ru-central1-d` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) рабочих NodeGroup:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
   ```

1. Исключите `ru-central1-f` и `ru-central1-d` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в [NodeGroup](/modules/node-manager/cr.html#nodegroup) `master`.
   При следующем применении с помощью `dhctl converge` команда последовательно пересоздаст master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений и после каждой замены выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Дождитесь применения изменений CloudEphemeral модулем `node-manager`.
   Для CloudPermanent примените изменения с помощью `dhctl converge`.
   Убедитесь, что узлы больше не используют `ru-central1-f` и `ru-central1-d`, а их подсети больше не используются другими ресурсами Yandex Cloud.

1. Выполните `d8 k edit mc cloud-provider-yandex` и временно укажите в [`zones`](configuration.html#parameters-nodes-parameters-zones) список без `ru-central1-f`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-e
   ```

1. Примените изменения с помощью `dhctl converge`.
   Дождитесь удаления подсети `ru-central1-f` и создания подсети `ru-central1-e`.
   Проверьте список подсетей:

   ```shell
   yc vpc subnet list
   ```

1. Добавьте `ru-central1-f` обратно в глобальный список [`zones`](configuration.html#parameters-nodes-parameters-zones):

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-e
             - ru-central1-f
   ```

1. Примените изменения с помощью `dhctl converge`.
   В этом примере Deckhouse Platform создаст подсеть `ru-central1-f` в CIDR-слоте `ru-central1-d`.

1. Добавьте `ru-central1-e` и `ru-central1-f` в [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) групп, которые должны использовать эти зоны.
   Для CloudPermanent снова примените изменения с помощью `dhctl converge`, для CloudEphemeral дождитесь применения модулем `node-manager`.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

Чтобы освободить CIDR-слот и пересоздать подсеть `ru-central1-f`, выполните следующие шаги:

1. Исключите `ru-central1-f` и `ru-central1-d` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) рабочих NodeGroup типа CloudEphemeral:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
   ```

1. Выполните `d8 system edit provider-cluster-configuration` и измените [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones), оставив `ru-central1-a` и `ru-central1-b`.
   Для master-узлов укажите тот же список в [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).
   При следующем применении с помощью `dhctl converge` команда последовательно пересоздаст master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений и после каждой замены выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Примените изменения постоянных групп с помощью `dhctl converge`.
   Дождитесь применения изменений CloudEphemeral модулем `node-manager`.
   Убедитесь, что узлы больше не используют `ru-central1-f` и `ru-central1-d`, а их подсети больше не используются другими ресурсами Yandex Cloud.

1. Снова выполните `d8 system edit provider-cluster-configuration` и временно укажите в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) список без `ru-central1-f`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-e
   ```

1. Примените изменения с помощью `dhctl converge`.
   Дождитесь удаления подсети `ru-central1-f` и создания подсети `ru-central1-e`.
   Проверьте список подсетей:

   ```shell
   yc vpc subnet list
   ```

1. Добавьте `ru-central1-f` обратно в глобальный список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-e
     - ru-central1-f
   ```

1. Примените изменения с помощью `dhctl converge`.
   В этом примере Deckhouse Platform создаст подсеть `ru-central1-f` в CIDR-слоте `ru-central1-d`.

1. Добавьте `ru-central1-e` и `ru-central1-f` в списки зон групп, которые должны использовать эти зоны.
   Для CloudEphemeral измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в NodeGroup.
   Для постоянных групп измените [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) или [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) и снова примените изменения с помощью `dhctl converge`.

{% endtab %}

{% endtabs %}

После выполнения выбранной процедуры проверьте узлы и подсети:

```shell
d8 k get nodes -L topology.kubernetes.io/zone
yc vpc subnet list
```

Убедитесь, что узлы используют только зоны из итогового списка.
При автоматическом создании подсетей проверьте, что подсеть `ru-central1-f` использует CIDR-слот исключённой зоны по умолчанию.

#### Вывод из эксплуатации зоны ru-central1-b

Чтобы прекратить размещение узлов в `ru-central1-b`, выберите итоговый список зон и воспользуйтесь соответствующей инструкцией ниже.

{% alert level="warning" %}

С 16 декабря 2026 года Yandex Cloud ограничивает создание и изменение виртуальных машин в зоне `ru-central1-b`. До этой даты исключите зону из конфигурации групп узлов и перенесите необходимые данные в другие зоны. Иначе Deckhouse Platform не сможет создавать и пересоздавать узлы в `ru-central1-b`.

{% endalert %}

Если используется [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap), сначала выведите узлы из `ru-central1-b`, исключив эту зону из списков зон групп.
Для CloudPermanent примените изменения групп с помощью `dhctl converge`, для CloudEphemeral дождитесь применения модулем `node-manager`.
Затем укажите оставшиеся зоны в глобальном [`zones`](configuration.html#parameters-nodes-parameters-zones), даже если раньше этот параметр не был задан, и примените изменения с помощью `dhctl converge`.
Запись `ru-central1-b` в [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) можно сохранить.
Удаляйте её только после вывода узлов и сохраняйте записи остальных зон, как описано в разделе [«Использование существующих подсетей»](#использование-существующих-подсетей).

Итоговый список может содержать любую комбинацию от одной до четырёх поддерживаемых зон.
В примерах кластер первоначально использует четыре зоны по умолчанию, а подсети создаёт Deckhouse Platform.
В YAML-фрагментах показаны только изменяемые поля. Сохраните остальные параметры существующих ресурсов.
В каждом примере выберите вкладку, соответствующую способу конфигурации кластера.

{% tabs zones_b_migration %}

{% tab "Исключить ru-central1-b без добавления новой зоны" %}

Выберите этот пример, чтобы продолжить работу в оставшихся зонах по умолчанию, например если квота в `ru-central1-f` ещё недоступна.
В итоговом списке останутся `ru-central1-a`, `ru-central1-d` и `ru-central1-e`.

{% tabs zones_b_keep_configuration %}

{% tab "ModuleConfig" %}

Для кластера с конфигурацией через ModuleConfig выполните следующие шаги:

1. Исключите `ru-central1-b` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в каждой рабочей [NodeGroup](/modules/node-manager/cr.html#nodegroup), использующей эту зону:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в [NodeGroup](/modules/node-manager/cr.html#nodegroup) `master`, указав тот же список зон без `ru-central1-b`.

1. Дождитесь завершения изменения групп узлов.
   Для `CloudEphemeral` модуль [`node-manager`](/modules/node-manager/) применяет изменение автоматически.
   Для групп типа `CloudPermanent`, включая `master`, примените изменения с помощью `dhctl converge`.
   Команда последовательно пересоздаёт master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений.
   После каждой замены master-узла выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Убедитесь, что в `ru-central1-b` больше нет узлов кластера:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   Команда не должна вывести имена узлов.

1. Откройте ModuleConfig командой `d8 k edit mc cloud-provider-yandex` и измените [`zones`](configuration.html#parameters-nodes-parameters-zones), исключив `ru-central1-b`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-d
             - ru-central1-e
   ```

1. Примените изменения с помощью `dhctl converge`.
   Deckhouse Platform сохранит подсети всех четырёх зон по умолчанию, включая `ru-central1-b`, поскольку в [`zones`](configuration.html#parameters-nodes-parameters-zones) останутся только зоны по умолчанию.

1. Проверьте результат:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   В списке узлов не должно быть `ru-central1-b`. Подсеть этой зоны должна оставаться в списке подсетей.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

Для кластера с устаревшим способом конфигурации через YandexClusterConfiguration выполните следующие шаги:

1. Исключите `ru-central1-b` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в каждой рабочей [NodeGroup](/modules/node-manager/cr.html#nodegroup) типа `CloudEphemeral`, использующей эту зону:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Выполните `d8 system edit provider-cluster-configuration` и исключите `ru-central1-b` из [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) постоянных рабочих групп и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones). Например, для рабочей группы `worker` и master-узлов укажите:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-d
       - ru-central1-e
   ```

1. Дождитесь завершения изменения групп узлов.
   Для `CloudEphemeral` модуль [`node-manager`](/modules/node-manager/) применяет изменение автоматически.
   Для групп типа `CloudPermanent`, включая `master`, примените изменения с помощью `dhctl converge`.
   Команда последовательно пересоздаёт master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений.
   После каждой замены master-узла выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Убедитесь, что в `ru-central1-b` больше нет узлов кластера:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   Команда не должна вывести имена узлов.

1. Снова выполните `d8 system edit provider-cluster-configuration` и измените глобальный список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones), исключив `ru-central1-b`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
   ```

1. Примените изменения с помощью `dhctl converge`.
   Deckhouse Platform сохранит подсети всех четырёх зон по умолчанию, включая `ru-central1-b`, поскольку в [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) останутся только зоны по умолчанию.

1. Проверьте результат:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   В списке узлов не должно быть `ru-central1-b`. Подсеть этой зоны должна оставаться в списке подсетей.

{% endtab %}

{% endtabs %}

{% endtab %}

{% tab "Заменить ru-central1-b на ru-central1-f" %}

Выберите этот пример, если квота в `ru-central1-f` доступна и нужна новая зона.
В итоговом списке будут `ru-central1-a`, `ru-central1-d`, `ru-central1-e` и `ru-central1-f`.

{% tabs zones_b_replace_configuration %}

{% tab "ModuleConfig" %}

Для кластера с конфигурацией через ModuleConfig выполните следующие шаги:

1. Исключите `ru-central1-b` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в каждой рабочей [NodeGroup](/modules/node-manager/cr.html#nodegroup), использующей эту зону:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Измените [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в [NodeGroup](/modules/node-manager/cr.html#nodegroup) `master`, указав тот же список зон без `ru-central1-b`.

1. Дождитесь завершения изменения групп узлов.
   Для `CloudEphemeral` модуль [`node-manager`](/modules/node-manager/) применяет изменение автоматически.
   Для групп типа `CloudPermanent`, включая `master`, примените изменения с помощью `dhctl converge`.
   Команда последовательно пересоздаёт master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений.
   После каждой замены master-узла выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Убедитесь, что в `ru-central1-b` больше нет узлов кластера:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   Команда не должна вывести имена узлов.
   Отдельно проверьте, что подсеть `ru-central1-b` больше не используется другими ресурсами Yandex Cloud.

1. Откройте ModuleConfig командой `d8 k edit mc cloud-provider-yandex` и измените [`zones`](configuration.html#parameters-nodes-parameters-zones), заменив `ru-central1-b` на `ru-central1-f`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-d
             - ru-central1-e
             - ru-central1-f
   ```

1. Примените изменения с помощью `dhctl converge`.
   Deckhouse Platform удалит подсеть `ru-central1-b` и создаст подсеть `ru-central1-f` в освободившемся CIDR-слоте.

1. Добавьте `ru-central1-f` в [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) групп, которые должны создавать узлы в новой зоне:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
         - ru-central1-f
   ```

   Для групп типа `CloudPermanent` примените изменения с помощью `dhctl converge`. Для `CloudEphemeral` дождитесь применения изменений модулем `node-manager`.

1. Проверьте результат:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   В списке узлов не должно быть `ru-central1-b`.
   В списке подсетей должна быть подсеть `ru-central1-f`, а подсети `ru-central1-b` быть не должно.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

Для кластера с устаревшим способом конфигурации через YandexClusterConfiguration выполните следующие шаги:

1. Исключите `ru-central1-b` из [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) в каждой рабочей [NodeGroup](/modules/node-manager/cr.html#nodegroup) типа `CloudEphemeral`, использующей эту зону:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Выполните `d8 system edit provider-cluster-configuration` и исключите `ru-central1-b` из [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) постоянных рабочих групп и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones). Например, для рабочей группы `worker` и master-узлов укажите:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-d
       - ru-central1-e
   ```

1. Дождитесь завершения изменения групп узлов.
   Для `CloudEphemeral` модуль [`node-manager`](/modules/node-manager/) применяет изменение автоматически.
   Для групп типа `CloudPermanent`, включая `master`, примените изменения с помощью `dhctl converge`.
   Команда последовательно пересоздаёт master-узлы, для которых изменились параметры размещения. Перед подтверждением изучите план изменений.
   После каждой замены master-узла выполните [проверки etcd и компонентов control plane](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#в-облачном-кластере).

1. Убедитесь, что в `ru-central1-b` больше нет узлов кластера:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   Команда не должна вывести имена узлов.
   Отдельно проверьте, что подсеть `ru-central1-b` больше не используется другими ресурсами Yandex Cloud.

1. Снова выполните `d8 system edit provider-cluster-configuration` и измените глобальный список [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones), заменив `ru-central1-b` на `ru-central1-f`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
     - ru-central1-f
   ```

1. Примените изменения с помощью `dhctl converge`.
   Deckhouse Platform удалит подсеть `ru-central1-b` и создаст подсеть `ru-central1-f` в освободившемся CIDR-слоте.

1. Добавьте `ru-central1-f` в [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) рабочих NodeGroup типа `CloudEphemeral`, которые должны создавать узлы в новой зоне:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
         - ru-central1-f
   ```

   Дождитесь применения изменений модулем `node-manager`.

1. Добавьте `ru-central1-f` в [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) и [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) групп, которые должны создавать узлы в новой зоне:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
         - ru-central1-f
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-d
       - ru-central1-e
       - ru-central1-f
   ```

   Примените изменения с помощью `dhctl converge`.

1. Проверьте результат:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   В списке узлов не должно быть `ru-central1-b`.
   В списке подсетей должна быть подсеть `ru-central1-f`, а подсети `ru-central1-b` быть не должно.

{% endtab %}

{% endtabs %}

{% endtab %}

{% endtabs %}

Если удаление узлов не завершается, проверьте состояние NodeGroup и сообщения об ошибках. Для CloudPermanent проверьте вывод `dhctl converge`. Изменяйте глобальный список зон только после удаления узлов.

Не удаляйте управляемые виртуальные машины и объекты Machine вручную, поскольку это может нарушить процесс изменения групп узлов.

## Пример кастомного ресурса YandexInstanceClass

YandexInstanceClass описывает виртуальные машины, которые Deckhouse Platform заказывает для группы узлов. Обязательны только два параметра, [`cores`](cr.html#yandexinstanceclass-v1-spec-cores) и [`memory`](cr.html#yandexinstanceclass-v1-spec-memory). Остальные параметры можно не задавать.

В примере ниже дополнительно заданы образ ОС, тип и размер диска, доля вычислительной мощности ядра и режим работы сетевого интерфейса:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: fd8bhu2b2bsbnqkhhd2k
  diskType: network-ssd
  diskSizeGB: 50
  networkType: SoftwareAccelerated
```

## LoadBalancer

### Аннотации объекта Service

В кластере заданы значения по умолчанию для размещения ресурсов балансировщиков (сеть для целевой группы и подсеть для Listener). Эти значения выставляются автоматически во время развёртывания кластера и могут быть переопределены аннотациями на уровне конкретного Service.

> Поведение по умолчанию (внешний или внутренний LB) зависит от конфигурации кластера. Для явного выбора типа используйте аннотацию `yandex.cpi.flant.com/loadbalancer-external`.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — указывает NetworkID, в котором будет создана целевая группа для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-subnet-id` — задаёт SubnetID для слушателей (listeners) создаваемого балансировщика для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-address-ipv4` — задаёт предопределённый IPv4-адрес для слушателей (поддерживаются и внутренние, и внешние LB).
1. `yandex.cpi.flant.com/loadbalancer-external` — включает создание внешнего (external) LB для данного Service (используйте, если нужно явно создать внешний балансировщик). Переопределяет поведение по умолчанию.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени целевой группы, которую использует LoadBalancer. Аннотация на Service определяет целевую группу, но не выбирает узлы. Чтобы включить узлы в целевую группу, задайте аннотацию с тем же значением в параметре [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations) ресурса NodeGroup. Yandex Cloud Controller Manager включает в целевую группу все подходящие узлы с таким значением аннотации независимо от их NodeGroup. Имя целевой группы формируется как `<ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>`. Подробнее — в разделе [«Использование отдельной целевой группы для NodeGroup»](faq.html#использование-отдельной-целевой-группы-для-nodegroup).

Если для control plane или master-узлов создаются отдельные целевые группы, добавьте на master-узлы лейбл `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые целевые группы для балансировщиков.

Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте целевую группу с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

### Проверки состояния целевой группы

Параметры проверок состояния для создаваемых целевых групп балансировщика:

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).
