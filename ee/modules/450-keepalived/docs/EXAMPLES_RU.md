---
title: "Модуль keepalived: примеры"
---

## Три публичных IP-адреса

Три публичных IP-адреса на трех front-узлах. Каждый виртуальный IP-адрес вынесен в отдельную VRRP-группу, таким образом, каждый адрес «прыгает» независимо от других, и если в кластере три узла с лейблами `node-role.deckhouse.io/frontend: ""`, то каждый IP получит по своему master-узлу.

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
      detectionStrategy: DefaultRoute # В качестве служебной сетевой карты используем ту, через которую проложен дефолтный маршрут.
    virtualIPAddresses:
    - address: 42.43.44.101/32
      # В нашем примере адреса «прыгают» по тем же сетевым интерфейсам, по которым ходит служебный VRRP-трафик, поэтому мы не указываем параметр interface.
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

Шлюз с парой IP-адресов для LAN и WAN. В случае шлюза приватный и публичный IP друг без друга не могут и «прыгать» между узлами они будут вместе. Служебный VRRP-трафик в данном примере мы решили пустить через LAN-интерфейс, который мы определим с помощью метода NetworkAddress (считаем, что на каждом узле есть IP из этой подсети).

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
  - id: 4 # ID "1", "2", "3" уже заняты в KeepalivedInstance "front" выше.
    interface:
      detectionStrategy: NetworkAddress
      networkAddress: 192.168.42.0/24
    virtualIPAddresses:
    - address: 192.168.42.1/24
      # В данном случае мы уже определили локальную сеть выше и можем не определять интерфейс для этого IP, не указав параметр interface.
    - address: 42.43.44.1/28
      interface:
        detectionStrategy: Name
        name: ens7 # На всех узлах интерфейс для публичных IP называется "ens7", воспользуемся этим.
```
