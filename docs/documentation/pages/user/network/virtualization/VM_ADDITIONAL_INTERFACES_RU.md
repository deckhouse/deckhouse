---
title: "Дополнительные сетевые интерфейсы виртуальных машин"
permalink: ru/user/network/virtualization/vm-additional-interfaces.html
description: "Дополнительные сетевые интерфейсы виртуальной машины из сетей модуля sdn и выделение для них IP-адресов."
search: дополнительные интерфейсы, SDN, ClusterNetwork, IPAddress, сети проекта
lang: ru
---

Кроме основной сети кластера виртуальную машину можно подключить к дополнительным сетям модуля [`sdn`](/modules/sdn/) и выдать её интерфейсам адреса.

## Дополнительные сетевые интерфейсы

Кроме основной сети кластера машину можно подключить к дополнительным сетям, проектным и кластерным.

{% alert level="warning" %}
Для работы с дополнительными сетями необходимо, чтобы модуль `sdn` был активирован.
{% endalert %}

Ниже показано, как подключить машину к дополнительной сети:

{% tabs vm-networks %}

{% tab "В командной строке" %}

Виртуальные машины могут быть подключены к дополнительным сетям — проектным (Network) или кластерным (ClusterNetwork).

Для этого перечислите нужные сети в блоке [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks). Если этот блок не задан (что является значением по умолчанию), ВМ будет использовать только основную сеть кластера.

> Основную сеть кластера (`type: Main`) в [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) указывать необязательно. Если вам не требуется подключение к основной сети кластера, можно использовать только дополнительные сети (`Network` или `ClusterNetwork`).
>
> Однако если основная сеть указана, она обязательно должна быть первой в списке [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks).

Особенности и важные моменты работы с дополнительными сетевыми интерфейсами:

- порядок перечисления сетей в [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) определяет порядок подключения интерфейсов внутри виртуальной машины;
- добавление или удаление дополнительной сети (`Network` или `ClusterNetwork`) на работающей ВМ применяется без перезагрузки. ACPI-индексы существующих интерфейсов сохраняются при добавлении и удалении, поэтому имена интерфейсов в гостевой ОС остаются стабильными;
- добавление или удаление основной сети (`type: Main`) по-прежнему требует перезагрузки ВМ, так как она связана с основным сетевым интерфейсом пода и не может быть изменена на работающем поде;
- чтобы сохранить порядок сетевых интерфейсов внутри гостевой операционной системы, добавляйте новые сети в конец списка [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) и не меняйте порядок существующих;
- политики сетевой безопасности (NetworkPolicy) не применяются к дополнительным сетевым интерфейсам;
- параметры сети (IP-адреса, шлюзы, DNS и так далее) для дополнительных сетей настраиваются вручную изнутри гостевой ОС (например, с помощью Cloud-Init), если для сети не настроен IPAM (подробнее — в подразделе [«IPAM для дополнительных сетевых интерфейсов»](#ipam-для-дополнительных-сетевых-интерфейсов)).

> При настройке сетевых интерфейсов в гостевой ОС используйте стабильные идентификаторы (предсказуемые имена `enpXsY` или привязку по MAC-адресу) вместо имён `ethX`, как описано в разделе [«Именование сетевых интерфейсов в гостевой ОС»](../../virtualization/vm-block-devices.html#именование-сетевых-интерфейсов-в-гостевой-ос).
>
> На гостевой системе Linux с несколькими интерфейсами в одной подсети может возникать проблема ARP Flux, при которой ядро отвечает на ARP-запросы через произвольный интерфейс, а не через тот, на который пришёл запрос, что приводит к нестабильному соединению и потере пакетов из-за некорректного MAC-адреса в кеше маршрутизаторов.
>
> Чтобы это исправить, установите параметры, которые заставляют систему отвечать на запросы строго через интерфейс с целевым IP и использовать корректный исходный адрес:
>
> ```bash
> sysctl -w net.ipv4.conf.all.arp_ignore=1
> sysctl -w net.ipv4.conf.all.arp_announce=2
> ```
>
> Пример для cloud-init:
>
> ```yaml
> write_files:
> - path: /etc/sysctl.d/90-arp-strict.conf
> content: |
> net.ipv4.conf.all.arp_ignore=1
> net.ipv4.conf.all.arp_announce=2
> ```
>
> Значения параметров описаны в [документации IP sysctl](https://docs.kernel.org/networking/ip-sysctl.html).

Пример подключения ВМ к основной сети кластера и проектной сети `user-net`:

```yaml
spec:
  networks:
    - type: Main # Если указана, должна быть первой
    - type: Network # Тип сети (Network \ ClusterNetwork)
      name: user-net # Название сети
```

Пример подключения к нескольким сетям, включая кластерную сеть `corp-net`:

```yaml
spec:
  networks:
    - type: Main # Если указана, должна быть первой
    - type: Network
      name: user-net
    - type: ClusterNetwork
      name: corp-net # Название сети
```

Пример подключения ВМ только к дополнительным сетям (без основной сети кластера):

```yaml
spec:
  networks:
    - type: Network
      name: isolated-net
    - type: ClusterNetwork
      name: corp-net
```

Информацию о подключённых сетях и их MAC-адресах можно посмотреть в статусе ВМ:

```yaml
status:
  networks:
    - type: Main
    - type: Network
      name: user-net
      macAddress: aa:bb:cc:dd:ee:01
    - type: ClusterNetwork
      name: corp-net
      macAddress: aa:bb:cc:dd:ee:02
```

Для каждого дополнительного сетевого интерфейса автоматически создаётся и резервируется уникальный MAC-адрес, что обеспечивает отсутствие коллизий MAC-адресов. Для этого служат ресурсы [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) (`vmmac`) и [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease) (`vmmacl`).

MAC-адрес генерируется случайным образом из пула разрешённых диапазонов.

- Диапазоны: `x2-xx-xx-xx-xx-xx`, `x6-xx-xx-xx-xx-xx`, `xA-xx-xx-xx-xx-xx`, `xE-xx-xx-xx-xx-xx`.
- Первые три октета (OUI) формируются на основе UUID кластера, последние три (NIC) — выбираются случайно из 16 миллионов возможных комбинаций.

Ресурс [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease) (`vmmacl`) — кластерный ресурс, который управляет арендой MAC-адресов из общего пула MAC-адресов.

Чтобы посмотреть список аренд MAC-адресов (`vmmacl`), используйте команду:

```bash
d8 k get vmmacl
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME                    VIRTUALMACHINEMACADDRESS                      STATUS   AGE
mac-5e-e6-19-22-0f-d8   {"name":"vm-01-fz9cr","namespace":"pr-sdn"}   Bound    45s
mac-5e-e6-19-29-89-cf   {"name":"vm-01-99qj6","namespace":"pr-sdn"}   Bound    45s
mac-5e-e6-19-54-f9-be   {"name":"vm-01-5jqxg","namespace":"pr-sdn"}   Bound    45s
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Ресурс [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) (`vmmac`) — проектный ресурс, который отвечает за резервирование арендованных MAC-адресов и их привязку к виртуальным машинам.

MAC-адрес назначается автоматически на каждый дополнительный интерфейс из общего пула адресов и закрепляется за ней до её удаления.

Проверить назначенные MAC-адреса можно с помощью команды:

```bash
d8 k get vmmac
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME          ADDRESS             STATUS     VM      AGE
vm-01-5jqxg   5e:e6:19:54:f9:be   Attached   vm-01   5m42s
vm-01-99qj6   5e:e6:19:29:89:cf   Attached   vm-01   5m42s
vm-01-fz9cr   5e:e6:19:22:0f:d8   Attached   vm-01   5m42s
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

При удалении сети из конфигурации ВМ:

- MAC-адрес интерфейса освобождается.
- Автоматически удаляются связанные ресурсы [VirtualMachineMACAddress](/modules/virtualization/cr.html#virtualmachinemacaddress) и [VirtualMachineMACAddressLease](/modules/virtualization/cr.html#virtualmachinemacaddresslease).
- Автоматически удаляется выделенный ресурс `IPAddress` (если использовался IPAM).

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
1. Из списка выберите нужную ВМ и нажмите на её имя.
1. На вкладке «Конфигурация» прокрутите страницу до раздела «Сети» и нажмите кнопку «Добавить».
1. В открывшемся окне «Добавить сеть» в поле «Выберите сеть» укажите нужную сеть.
1. Нажмите кнопку «Добавить», затем — появившуюся кнопку «Сохранить».

Чтобы создать сеть проекта:

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Сеть» → «SDN» → «Сети».
1. Нажмите кнопку «Создать».
1. В открывшемся окне «Создать ресурс» в поле «Имя» введите имя сети.
1. На вкладке «Конфигурация» в поле «Сетевой класс» выберите Network-класс, в поле «Type» — тип сети, в поле «VLAN» — идентификатор VLAN. При необходимости задайте «Mtu» и параметры блока «IPAM».
1. Нажмите кнопку «Применить».
1. Созданные сети отображаются в списке с колонками «Статус», «Тип», «VLAN» и «Network-класс».

{% endtab %}

{% endtabs %}

## IPAM для дополнительных сетевых интерфейсов

Адреса в дополнительной сети модуль может выдавать сам, если администратор настроил для этой сети пул адресов.

{% tabs net-ipam %}

{% tab "В командной строке" %}

Если [в модуле `sdn`](/modules/sdn/) для дополнительной сети настроен IPAM (пул IP-адресов, привязанный к сети через [`spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#clusternetwork-v1alpha1-spec-ipam-ipaddresspoolref)), модуль `virtualization` может автоматически выделять IP-адреса для дополнительных интерфейсов ВМ и доставлять их в гостевую ОС через DHCP.

Поддерживаются два режима:

- **Автоматический (DHCP)** — если в [`.spec.networks[]`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) не указано [поле `ipAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks-ipaddressname), контроллер автоматически создаёт ресурс IPAddress (тип `Auto`), привязанный к ВМ через `ownerReferences`, и передаёт его в модуль `sdn`. Модуль `sdn` выделяет адрес из пула и доставляет его в гостевую ОС через DHCP. Адрес сохраняется при перезагрузках и миграции ВМ, поскольку привязан к ВМ, а не к поду. Для работы этого режима в гостевой ОС на соответствующем интерфейсе должен быть включён DHCP-клиент.

- **Статический** — если в [`.spec.networks[]`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) указано [поле `ipAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks-ipaddressname), контроллер использует предоставленный пользователем ресурс IPAddress (тип `Static`, `network.deckhouse.io/v1alpha1`). Адрес определяется пользователем и не изменяется автоматически.

Если у дополнительной сети не настроен пул IPAM, функция IPAM не включается — интерфейс работает в режиме L2-only, а IP-адресацию необходимо настроить вручную в гостевой ОС.

Пример конфигурации ВМ с автоматическим выделением IP-адреса для дополнительной сети:

```yaml
spec:
  networks:
    - type: Main
    - type: ClusterNetwork
      name: corp-net
      # ipAddressName не указан → используется автоматический режим (DHCP)
```

Пример конфигурации ВМ со статическим IP-адресом для дополнительной сети:

```yaml
spec:
  networks:
    - type: Main
    - type: ClusterNetwork
      name: corp-net
      ipAddressName: my-static-ip # Имя ресурса IPAddress (SDN)
```

Пример конфигурации статического ресурса IPAddress:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: IPAddress
metadata:
  name: my-static-ip
  namespace: my-namespace
spec:
  networkRef:
    kind: ClusterNetwork
    name: corp-net
  type: Static
  static:
    ip: 192.168.200.42
```

Выделенный IP-адрес отображается в статусе ВМ:

```yaml
status:
  ipAddress: 10.66.10.2                     # IP-адрес основной сети.
  virtualMachineIPAddressName: vm-01-main-ip # Имя IPAddress основной сети.
  networks:
    - type: Main
    - type: ClusterNetwork
      name: corp-net
      macAddress: 32:a6:a1:0a:92:48
      virtualMachineMACAddressName: vm-01-rxzd6
      ipAddress: 192.168.200.4               # IP-адрес дополнительной сети (из IPAM).
```

> **Важно:** Если для дополнительной сети настроен пул IPAM, не настраивайте статический IP-адрес на дополнительном интерфейсе в гостевой ОС вручную (через Cloud-Init). Используйте автоматический (DHCP) или статический (`ipAddressName`) режим, чтобы избежать конфликтов адресов.
>
> Если у дополнительной сети есть пул IPAM, но ресурс IPAddress ещё не выделен или находится в состоянии `Pending` (например, из-за исчерпания пула адресов), интерфейс временно пропускается — ВМ запускается без него, а в condition `NetworkReady` сообщается об ошибке. После появления доступного IP-адреса интерфейс подключается автоматически с помощью механизма hotplug.

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Сеть» → «SDN» → «IP-пулы».
1. Нажмите кнопку «Создать».
1. В открывшемся окне «Создать ресурс» в поле «Имя» введите имя пула.
1. На вкладке «Конфигурация» в поле «Lease TTL» задайте время жизни аренды, а в блоке «Pools» — сеть («Network»), диапазоны адресов («Ranges») и маршруты («Routes»).
1. Нажмите кнопку «Применить».

{% endtab %}

{% endtabs %}

### Настройка гостевой ОС для интерфейсов, добавленных на ходу

Когда дополнительный сетевой интерфейс подключают к уже запущенной ВМ, гостевая ОС должна быть настроена на автоматический подъём новых сетевых интерфейсов и запрос DHCP-аренды. Linux по умолчанию не запускает DHCP-клиент на интерфейсах, добавленных на ходу.

Чтобы такие интерфейсы настраивались сами, используйте в гостевой ОС один из следующих подходов:

- **NetworkManager** (Ubuntu, RHEL, CentOS) — автоматически настраивает новые интерфейсы с DHCP, если запущен сервис `network-manager`;
- **udev-правило** (Alpine и другие системы без `network-manager`) — добавьте udev-правило для подъёма новых интерфейсов:

  ```yaml
  write_files:
    - path: /etc/udev/rules.d/90-hotplug-network.rules
      content: |
        SUBSYSTEM=="net", ACTION=="add", RUN+="/sbin/ifup %k"
  ```

Для интерфейсов, присутствующих при загрузке ВМ (включённых в начальную сетевую конфигурацию), дополнительная настройка не требуется — гостевая ОС настраивает их при запуске через Cloud-Init.
