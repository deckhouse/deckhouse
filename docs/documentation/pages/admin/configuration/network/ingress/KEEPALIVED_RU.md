---
title: "Обеспечение высокой доступности и отказоустойчивости (keepalived)"
permalink: ru/admin/configuration/network/ingress/keepalived.html
description: "Настройка keepalived для высокой доступности в Deckhouse Kubernetes Platform. Конфигурация отказоустойчивости и настройка сетевой избыточности для инфраструктуры кластера."
lang: ru
---

В Deckhouse Kubernetes Platform для обеспечения высокой доступности и отказоустойчивости можно использовать модуль [`keepalived`](/modules/keepalived/).

<!-- Перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/keepalived/ -->

Для настройки keepalived-кластеров на узлах с его помощью используются Custom Resources (кастомные ресурсы).

## Примеры использования модуля

<!-- Перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/keepalived/examples.html -->

### Несколько публичных IP-адресов

Рассмотрим пример, когда три публичных IP-адреса расположены на трех front-узлах. Каждый виртуальный IP-адрес привязан к отдельной VRRP-группе, что обеспечивает независимое перемещение каждого адреса. При наличии в кластере трёх узлов с лейблами `node-role.deckhouse.io/frontend: ""`, каждый IP-адрес будет закреплён за своим master-узлом.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front
spec:
  nodeSelector: # Обязательно.
    node-role.deckhouse.io/frontend: ""
  tolerations:  # Опционально.
  - key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
  vrrpInstances:
  - id: 1 # Уникальный для всего кластера ID.
    interface:
      detectionStrategy: DefaultRoute # В качестве служебной сетевой карты используется та, через которую проложен дефолтный маршрут.
    virtualIPAddresses:
    - address: 42.43.44.101/32
      # В приведённом примере IP-адреса переходят по тем же сетевым интерфейсам, по которым передаётся служебный VRRP-трафик, поэтому параметр interface указывать не требуется.
  - id: 2
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 42.43.44.102/32
  - id: 3
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 42.43.44.103/32
```

Шлюз использует пару IP-адресов — для LAN и WAN. В отличие от других случаев, приватный и публичный IP-адреса связаны между собой и перемещаются между узлами совместно. В данном примере служебный VRRP-трафик передаётся через LAN-интерфейс, который определяется методом NetworkAddress (предполагается, что на каждом узле имеется IP-адрес из соответствующей подсети).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: mygateway
spec:
  nodeSelector:
    node-role.deckhouse.io/mygateway: ""
  tolerations:
  - key: node-role.deckhouse.io/mygateway
    operator: Exists
  vrrpInstances:
  - id: 4 # ID "1", "2", "3" уже используются в KeepalivedInstance "front" выше.
    interface:
      detectionStrategy: NetworkAddress
      networkAddress: 192.168.42.0/24
    virtualIPAddresses:
    - address: 192.168.42.1/24
      # Так как локальная сеть уже определена выше, параметр interface для этого IP можно не указывать.
    - address: 42.43.44.1/28
      interface:
        detectionStrategy: Name
        name: ens7 # Интерфейс для публичных IP на всех узлах называется "ens7", указываем его явно.
```

## Ручное переключение keepalived

<!-- перенесено из https://deckhouse.ru/modules/keepalived/ -->

1. Зайдите в нужный под:

   ```shell
   d8 k -n d8-keepalived exec -it keepalived-<name> -- sh
   ```

1. Отредактируйте файл `vi /etc/keepalived/keepalived.conf`, где в строке с параметром `priority` замените значение на число подов keepalived + 1.

1. Отправьте сигнал на перечитывание конфигурации:

   ```shell
   kill -HUP 1
   ```
