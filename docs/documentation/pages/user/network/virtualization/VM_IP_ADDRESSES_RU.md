---
title: "IP-адреса виртуальных машин"
permalink: ru/user/network/virtualization/vm-ip-addresses.html
description: "IP-адреса виртуальных машин: запрос конкретного адреса, сохранение адреса за проектом и общий IPAM основной сети."
search: IP-адрес ВМ, VirtualMachineIPAddress, IPAM, основная сеть
lang: ru
---

Каждая виртуальная машина получает адрес в основной сети кластера. Ниже описано, как посмотреть выданный адрес, запросить конкретный и сохранить его за проектом.

## IP-адреса ВМ

Адрес машины в основной сети кластера описывают два ресурса, аренда адреса в кластере и закреплённый за проектом адрес.

{% tabs vmip-list %}

{% tab "В командной строке" %}

Блок [`.spec.settings.virtualMachineCIDRs`](../../../admin/configuration/network/vm-network.html) в настройках модуля задаёт подсети, из которых машины получают IP-адреса. Доступны все адреса подсети, кроме первого и последнего.

Если в модуле [`sdn`](/modules/sdn/) для основной сети кластера настроен пул адресов, адресом машины в этой сети управляет общий IPAM этого модуля, то есть тот же ресурс IPAddress, что и для дополнительных сетей. Адрес запрашивается автоматически, а существующие машины переходят на общий IPAM без смены адресов и без перезапуска.

> **Важно:** В кластере с общим IPAM ресурсы [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) и [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease), а также параметр [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) устарели. Они продолжают работать и описаны в разделах ниже, но адресом основной сети новых машин управляет общий IPAM, о котором рассказывает раздел [IPAM для основной сети](#ipam-для-основной-сети).

Ресурс [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease) (`vmipl`) — кластерный ресурс, который управляет арендой IP-адресов из общего пула, указанного в `virtualMachineCIDRs`.

Чтобы посмотреть список аренд IP-адресов (`vmipl`), используйте команду:

```bash
d8 k get vmipl
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME             VIRTUALMACHINEIPADDRESS                             STATUS   AGE
ip-10-66-10-14   {"name":"linux-vm-7prpx","namespace":"default"}     Bound    12h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) (`vmip`) — проектный ресурс, который отвечает за резервирование арендованных IP-адресов и их привязку к виртуальным машинам. IP-адреса могут выделяться автоматически или по явному запросу.

Адрес закреплён за машиной, когда ресурс переходит в фазу `Attached`. Остальные фазы описаны в поле [`.status.phase`](/modules/virtualization/cr.html#virtualmachineipaddress-v1alpha2-status-phase).

По умолчанию DP назначает машине адрес сам и держит его закреплённым до удаления машины. Посмотреть назначенный адрес можно командой:

```bash
d8 k get vmip
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```console
NAME             ADDRESS       STATUS     VM         AGE
linux-vm-7prpx   10.66.10.14   Attached   linux-vm   12h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Алгоритм автоматического присвоения IP-адреса виртуальной машине выглядит следующим образом:

- Пользователь создаёт виртуальную машину с именем `<VM_NAME>`.
- DP автоматически создаёт ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) с именем `<VM_NAME>-<HASH>`, чтобы запросить IP-адрес и связать его с виртуальной машиной.
- Для этого [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) создаётся ресурс аренды [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease), который выбирает случайный IP-адрес из общего пула.
- Как только ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) создан, виртуальная машина получает назначенный IP-адрес.

После удаления машины ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) тоже удаляется, но сам адрес какое-то время остаётся закреплённым за проектом, и его можно запросить повторно.

Все параметры этих ресурсов описаны в [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) и [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease).

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «IP адреса».
1. В списке отображаются имя ресурса, статус, адрес, тип (`Auto` или `Static`), виртуальная машина, которая использует адрес, и возраст ресурса.

{% endtab %}

{% endtabs %}

### Назначение конкретного IP-адреса

Вместо случайного адреса из пула машине можно выдать адрес, выбранный заранее.

{% tabs vmip-static %}

{% tab "В командной строке" %}

1. Создайте ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress):

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachineIPAddress
   metadata:
     name: linux-vm-custom-ip
   spec:
     staticIP: 10.66.20.77
     type: Static
   EOF
   ```

1. Создайте новую или измените существующую виртуальную машину и в спецификации укажите требуемый ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) явно:

   ```yaml
   spec:
     virtualMachineIPAddressName: linux-vm-custom-ip
   ```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Перейдите на вкладку «Проекты» и выберите нужный проект.
1. Перейдите в раздел «Виртуализация» → «IP адреса».
1. Нажмите кнопку «Создать».
1. В открывшемся окне «Создать ресурс» в поле «Имя» введите имя ресурса.
1. На вкладке «Конфигурация» в поле «Тип» выберите `Static`, а в поле «Статический IP-адрес» укажите нужный адрес.
1. Нажмите кнопку «Применить».
1. Укажите имя созданного ресурса в параметре [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) виртуальной машины.

{% endtab %}

{% endtabs %}

### Сохранение IP-адреса за проектом

Чтобы автоматически выданный ip-адрес виртуальной машины не удалился вместе с самой виртуальной машиной выполните следующие действия.

Получите название ресурса [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) для заданной виртуальной машины:

```bash
d8 k get vm linux-vm -o jsonpath="{.status.virtualMachineIPAddressName}"
```

Пример вывода:

```console
linux-vm-7prpx
```

Удалите блок `.metadata.ownerReferences` из найденного ресурса:

```bash
d8 k patch vmip linux-vm-7prpx --type=merge --patch '{"metadata":{"ownerReferences":null}}'

# Или внесите аналогичные изменения, отредактировав ресурс.

d8 k edit vmip linux-vm-7prpx
```

После удаления виртуальной машины ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) сохранится, и его можно будет переиспользовать снова во вновь созданной виртуальной машине:

```yaml
spec:
  virtualMachineIPAddressName: linux-vm-7prpx
```

Даже если ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) удалить, IP-адрес остаётся арендованным за проектом ещё 10 минут, и его можно занять снова:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineIPAddress
metadata:
  name: linux-vm-custom-ip
spec:
  staticIP: 10.66.20.77
  type: Static
EOF
```

## IPAM для основной сети

{% alert level="warning" %}
Общий IPAM работает, только если в модуле [`sdn`](/modules/sdn/) для основной сети кластера настроен пул адресов. Без него адресами машин управляют устаревшие ресурсы [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress), описанные выше.
{% endalert %}

Адрес машины в основной сети хранит ресурс [IPAddress](/modules/sdn/cr.html#ipaddress) модуля `sdn`, тот же самый, которым адресуются дополнительные сети. Получить адрес можно двумя способами:

- **Автоматический** — если для сети `Main` не задано [поле `ipAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks-ipaddressname), контроллер создаёт ресурс IPAddress, привязанный к машине, и дожидается адреса до её запуска. Машина, которая не смогла получить адрес, остаётся в фазе `Pending`, а причину показывает условие `VirtualMachineIPAddressReady`.

- **Статический** — создайте ресурс IPAddress основной сети самостоятельно и укажите его в параметре [`.spec.networks[]`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks). Такой ресурс принадлежит вам, поэтому не удаляется вместе с машиной и может быть переназначен другой.

Пример статического ресурса IPAddress для основной сети:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: IPAddress
metadata:
  name: linux-vm-main-ip
spec:
  networkRef:
    kind: ClusterNetwork
    name: main
  type: Static
  static:
    ip: 10.66.20.77
EOF
```

Пример ссылки на этот ресурс в спецификации машины:

```yaml
spec:
  networks:
    - type: Main
      ipAddressName: linux-vm-main-ip
```

Изменение поля `ipAddressName` основной сети меняет адрес основного интерфейса, поэтому требует перезапуска машины.

Текущий адрес и имя ресурса IPAddress, за которым он закреплён, показывает статус машины:

```bash
d8 k get vm linux-vm -o jsonpath='{.status.networks[?(@.type=="Main")]}'
```

Пример вывода:

<!-- markdownlint-disable MD031 -->
```txt
{"id":1,"ipAddress":"10.66.10.14","ipAddressName":"linux-vm-4bkqr","type":"Main"}
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

### Перевод машины на общий IPAM

Машины, которые не ссылаются на ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) явно, переходят на общий IPAM автоматически, при этом адрес не меняется и машина не перезапускается.

Машина с заданным параметром [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) продолжает работать по устаревшему механизму, поскольку ссылка на ресурс задана в её спецификации явно.

Для ресурса [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress), указанного в спецификации машины, DP поддерживает ресурс IPAddress с тем же именем и тем же адресом. Чтобы перевести машину и сохранить её адрес, замените один параметр другим:

```yaml
spec:
  # Удалите параметр virtualMachineIPAddressName.
  networks:
    - type: Main
      # Ресурс IPAddress с тем же именем и адресом.
      ipAddressName: linux-vm-custom-ip
```

Перезапустите машину, чтобы применить изменение. Адрес останется прежним, а ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) после этого можно удалить.

{% alert level="warning" %}
Не удаляйте параметр [`.spec.virtualMachineIPAddressName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineipaddressname) без такой замены, иначе машина получит новый адрес из пула.
{% endalert %}
