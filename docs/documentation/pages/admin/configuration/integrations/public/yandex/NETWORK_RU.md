---
title: Настройка сети в Yandex Cloud
permalink: ru/admin/integrations/public/yandex/network.html
lang: ru
---

## Сетевые параметры и безопасность

Далее описаны настройки, связанные с адресацией, маршрутизацией, внешним трафиком и безопасностью сети в кластере Deckhouse Platform, развернутом в Yandex Cloud.

### Внутренняя адресация узлов кластера

[Параметр `nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) задаёт диапазон IP-адресов, который распределяется между [зонами доступности](../../../../glossary.html#availability-zone) и применяется к внутренним интерфейсам узлов. Параметр задаётся в секции `nodes.parameters` ресурса [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        nodeNetworkCIDR: 192.168.12.0/24
```

При автоматическом создании подсетей Deckhouse Platform делит [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) на четыре равные части для внутренних подсетей узлов. За каждой зоной по умолчанию закреплена своя часть диапазона:

| Зона | Часть диапазона |
| --- | --- |
| `ru-central1-a` | Первая |
| `ru-central1-b` | Вторая |
| `ru-central1-e` | Третья |
| `ru-central1-d` | Четвёртая |

В [параметре `zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) можно явно указать от одной до четырёх поддерживаемых зон. При автоматическом создании подсетей Deckhouse Platform создаёт или сохраняет подсети всех четырёх зон по умолчанию, если [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) не задан, пуст или содержит только зоны по умолчанию.

При установке кластера и изменении его конфигурации можно выбрать только `ru-central1-f`, без зон по умолчанию. Правила выбора зон и пример настройки приведены [в разделе «Выбор зон для кластера»](/modules/cloud-provider-yandex/examples.html#выбор-зон-для-кластера).

Перед изменением зон ознакомьтесь с последствиями, выполните подготовку и выберите инструкцию для способа конфигурации кластера в разделе [«Изменение состава зон в работающем кластере»](/modules/cloud-provider-yandex/examples.html#изменение-состава-зон-в-работающем-кластере). Проверяйте квоты во всех зонах из временных и итоговых списков зон групп узлов, включая уже используемые зоны.

{% alert level="warning" %}
С 16 декабря 2026 года Yandex Cloud ограничивает создание и изменение виртуальных машин в зоне `ru-central1-b`. До этой даты исключите зону из конфигурации групп узлов и перенесите необходимые данные в другие зоны. Иначе Deckhouse Platform не сможет создавать и пересоздавать узлы в `ru-central1-b`. Пошаговые примеры приведены в разделе [«Вывод из эксплуатации зоны ru-central1-b»](/modules/cloud-provider-yandex/examples.html#вывод-из-эксплуатации-зоны-ru-central1-b).
{% endalert %}

Узлы подключаются к подсети соответствующей зоны. При автоматическом создании подсетей диапазон делится на четыре части во всех [схемах размещения](../../../../glossary.html#layout). Если [параметр `existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) содержит записи, Deckhouse Platform использует указанные подсети, но не управляет ими. В этом случае при незаданном или пустом [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) используются зоны из [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Сохраняйте хотя бы одну запись в этом параметре, иначе Deckhouse Platform перейдёт к автоматическому созданию подсетей.

{% alert level="info" %}
К подсети привязывается только одна таблица маршрутизации, поэтому два кластера с модулем [`cni-simple-bridge`](/modules/cni-simple-bridge/) не могут использовать одни и те же подсети. Чтобы развернуть такие кластеры в общих подсетях, используйте модуль [`cni-cilium`](/modules/cni-cilium/).
{% endalert %}

### Назначение внешних IP-адресов и исходящего трафика

[Параметр `externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) задаётся в секции `nodes.parameters`
ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) отдельно для каждой [группы узлов](../../../../glossary.html#node-group):

- ключ — имя группы узлов;
- значение — список идентификаторов подсетей Yandex Cloud для внешних сетевых интерфейсов узлов CloudPermanent. Подсеть каждого узла должна соответствовать его зоне.

Задайте этот параметр, если узлам CloudPermanent нужны дополнительные внешние сетевые интерфейсы.
Если параметр не задан, [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) позволяет назначать публичные IP-адреса Yandex Cloud.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        externalSubnetIDs:
          worker:
            - <RU_CENTRAL1_A_SUBNET_ID>
            - <RU_CENTRAL1_B_SUBNET_ID>
            - <RU_CENTRAL1_D_SUBNET_ID>
```

Параметр [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) задаёт список внешних IP-адресов для каждой группы узлов CloudPermanent. Адрес из каждого элемента списка используется узлом с соответствующим индексом.

Если необходимо подключаться к master-узлу напрямую по публичному IP-адресу, укажите для него параметры [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses)
и [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids)
в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/). Учитывайте, что интерфейс из сети, указанной в `externalSubnetIDs`, будет использоваться как шлюз по умолчанию для узла.

### Настройка DNS и DHCP для внутренних сетей

[Параметр `dhcpOptions`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-dhcpoptions) задаёт настройки DHCP-сервера, которые применяются ко всем подсетям,
создаваемым в кластере Deckhouse Platform в Yandex Cloud. Параметр задаётся в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

Доступные поля:

- `domainName` — поисковый домен (search domain), который устанавливается в конфигурации сети.
- `domainNameServers` — массив IP-адресов DNS-серверов, которые используются как рекурсивные резолверы.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - 192.168.0.2
            - 192.168.0.3
```

{% alert level="info" %}
Указанные DNS-серверы в `domainNameServers` обязательно должны разрешать внешние и внутренние доменные зоны, используемые в кластере. В противном случае возможны сбои в работе сервисов.
{% endalert %}

После внесения изменений в `dhcpOptions`:

- принудительно обновите DHCP lease на всех виртуальных машинах;
- перезапустите компоненты, работающие в сети узла (`hostNetwork: true`), в том числе `kube-dns`, чтобы пересчитать содержимое `resolv.conf`.

Для применения изменений можно использовать:

```shell
netplan apply
```

или другой способ, принятый в вашей системе, например `systemd-networkd` или `dhclient`.

### Использование заранее созданных подсетей

Если вы заранее создали подсети в Yandex Cloud и хотите использовать их для узлов кластера, укажите соответствие зон и идентификаторов подсетей в [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Deckhouse Platform использует эти подсети, но не управляет ими. Для каждой зоны можно указать только один идентификатор подсети.

В следующем фрагменте ModuleConfig задано пять существующих подсетей, а [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) ограничивает общий список зонами `ru-central1-a` и `ru-central1-f`. Сохраните остальные параметры конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
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

Подсети `ru-central1-b`, `ru-central1-d` и `ru-central1-e` остаются в [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) и сохраняются в Yandex Cloud. В приведённой конфигурации их зоны не входят в общий список зон для размещения узлов.

Чтобы размещать новые узлы в одной из этих подсетей, включите её зону в [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones), сохранив в списке не более четырёх зон. Если в нужных группах узлов явно заданы списки зон, добавьте в них ту же зону.

Если [`zones`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones) не задан или пуст, общий список включает все пять зон из [`existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

Добавление подсетей и настройка списков зон групп узлов описаны [в разделе «Использование существующих подсетей»](/modules/cloud-provider-yandex/examples.html#использование-существующих-подсетей).

{% alert level="info" %}
Deckhouse Platform создаёт таблицу маршрутизации, но не привязывает её к подсетям. Привяжите таблицу вручную в интерфейсе Yandex Cloud.
{% endalert %}

### Дополнительные внешние сети

[Параметр `additionalExternalNetworkIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-ccm-parameters-additionalexternalnetworkids) задаёт список дополнительных внешних сетей,
адреса из которых Deckhouse Platform считает публичными. Параметр задаётся в секции `ccm.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

Эта настройка полезна, если:

- имеются внешние подсети, не указанные в `externalSubnetIDs`, но требующие учёта как внешние;
- требуется точный контроль над интерпретацией IP-адресов при настройке балансировки, маршрутизации и экспорта статуса;
- используются нестандартные схемы подключения, например через внешние NAT-шлюзы или ручное резервирование IP.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4sno
```

Если параметр не задан, Deckhouse Platform будет использовать только те подсети, что явно указаны в настройках модуля (например, через `externalSubnetIDs`), чтобы определять публичность IP.

## Настройка NAT-инстанса

Для размещения NAT-инстанса в существующей подсети используйте
[параметр `withNATInstance.internalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid). Инстанс создаётся в зоне, которой принадлежит эта подсеть.

Если необходимо создать новую подсеть, укажите [параметр `withNATInstance.internalSubnetCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetcidr). NAT-инстанс размещается в созданной подсети.

Задайте один из двух параметров, `withNATInstance.internalSubnetID` или `withNATInstance.internalSubnetCIDR`.

Если также указан [`withNATInstance.externalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-externalsubnetid), NAT-инстанс будет подключён к этой подсети через вторичный сетевой интерфейс.

## Настройка групп безопасности в Yandex Cloud

Для каждой [облачной сети](https://cloud.yandex.ru/ru/docs/vpc/concepts/network#network) Yandex Cloud создаёт [группу безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups)
по умолчанию, в том числе для сети кластера. Эта группа разрешает любой входящий и исходящий трафик и применяется к тем сетевым интерфейсам виртуальных машин, которым не назначена другая группа.

Ограничить трафик кластера можно правилами этой группы. Назначить узлам другую группу безопасности нельзя, параметров для этого нет ни в настройках модуля, ни в ресурсе YandexInstanceClass.

{% alert level="warning" %}
Не назначайте группы безопасности узлам вручную в консоли Yandex Cloud. Команда `dhctl converge` приводит виртуальные машины к состоянию, описанному в конфигурации кластера, и может снять такую привязку.
{% endalert %}

{% alert level="danger" %}
Не удаляйте правила, разрешающие любой трафик, пока не настроите остальные правила группы. Иначе группа заблокирует трафик между узлами и компоненты кластера потеряют связь друг с другом.
{% endalert %}

Ошибки в правилах нарушают работу кластера. Перед настройкой правил в production-кластере изучите [особенности работы групп безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups#security-groups-notes) в Yandex Cloud.

Если в кластере используется Cilium с туннелированием трафика подов через VXLAN, разрешите UDP-трафик между узлами на портах, которые перечислены в разделе [«Сетевое взаимодействие компонентов Deckhouse Platform»](../../../../reference/network_interaction.html).

Чтобы настроить правила группы безопасности:

1. Определите облачную сеть, в которой работает кластер Deckhouse Platform.

   Название сети совпадает с параметром [`prefix`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-prefix) ModuleConfig `global`.
   Его можно узнать с помощью команды:

   ```bash
   d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
   ```

   Если значение пустое, возьмите его из устаревшего параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) ресурса [ClusterConfiguration](../../../../glossary.html#cluster-configuration) (префикс перенесён в ModuleConfig `global`):

   ```bash
   d8 k get secrets -n kube-system d8-cluster-configuration -ojson | \
     jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
   ```

1. В консоли Yandex Cloud выберите сервис Virtual Private Cloud и перейдите в раздел «Группы безопасности». У вас должна отображаться одна группа безопасности с пометкой `Default`.

   ![Группа безопасности по умолчанию](../../../../images/cloud-provider-yandex/sg-ru-default.png)

1. Создайте правила согласно [инструкции Yandex Cloud](https://cloud.yandex.ru/ru/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

   ![Правила для группы безопасности](../../../../images/cloud-provider-yandex/sg-ru-rules.png)

1. Удалите правило, разрешающее любой **входящий** трафик (на скриншоте выше оно уже удалено), и сохраните изменения.

## Настройка доступа через bastion-хост

Для подключения к узлам, находящимся в приватных подсетях (например, при использовании [схемы размещения](../../../../glossary.html#layout) Standard или WithNATInstance),
используется [bastion-хост](../../../../glossary.html#bastion-host), промежуточная машина с публичным IP-адресом, через которую выполняется SSH-доступ к узлам.

NAT-инстанс не используется инсталлятором `dhctl` автоматически в качестве bastion-хоста или jump-хоста. Для установки кластера на master-узел должен быть SSH-доступ с машины, на которой запускается `dhctl`: напрямую или через bastion-хост.

Если master-узел недоступен напрямую, запускайте установку с параметрами `--ssh-bastion-host`, `--ssh-bastion-user` и, при необходимости, `--ssh-bastion-port`.

Для настройки доступа выполните следующие шаги:

1. Выполните bootstrap базовой инфраструктуры. Перед созданием bastion-хоста необходимо выполнить начальную фазу установки Deckhouse Platform, которая подготовит сетевую инфраструктуру:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

1. Создайте bastion-хост в Yandex Cloud:

   ```shell
   yc compute instance create \
     --name bastion \
     --hostname bastion \
     --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
     --memory 2 \
     --cores 2 \
     --core-fraction 100 \
     --ssh-key ~/.ssh/<SSH_PUBLIC_KEY_FILE> \
     --zone ru-central1-a \
     --public-address 178.154.226.159
   ```

   Замените `<SSH_PUBLIC_KEY_FILE>` на имя публичного ключа, например `id_rsa.pub` для ключа RSA или `id_ed25519.pub` для ключа ED25519.

   Убедитесь, что IP-адрес из параметра `--public-address` доступен из вашей сети и указан корректно.

1. Запустите основной bootstrap Deckhouse Platform через bastion-хост:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
     --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> --config=/config.yml
   ```

   Замените `<SSH_PRIVATE_KEY_FILE>` на имя приватного ключа, например `id_rsa` для ключа RSA или `id_ed25519` для ключа ED25519.

   Здесь:

   - `--ssh-bastion-user` — пользователь для подключения к bastion-хосту;
   - `--ssh-user` — пользователь на целевых узлах кластера;
   - `--ssh-agent-private-keys` — путь до приватного SSH-ключа;
   - `--config` — путь до конфигурационного файла Deckhouse Platform.
