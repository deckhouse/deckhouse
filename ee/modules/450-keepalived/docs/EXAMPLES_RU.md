---
title: "Модуль keepalived: примеры"
---

## Три публичных IP-адреса

Три публичных IP-адреса на трёх front-узлах. Каждый виртуальный IP-адрес вынесен в отдельную VRRP-группу, таким образом, каждый адрес "прыгает" независимо от других и если в кластере три узла с лейблами `node-role.deckhouse.io/frontend: ""`, то каждый IP получит по своему MASTER-узлу.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front
spec:
  nodeSelector: # обязательно
    node-role.deckhouse.io/frontend: ""
  tolerations:  # опционально
  - key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
  vrrpInstances:
  - id: 1 # уникальный для всего кластера id
    interface:
      detectionStrategy: DefaultRoute # в качестве служебной сетевой карты используем ту, через которую проложен дефолтный маршрут
    virtualIPAddresses:
    - address: 42.43.44.101/32
      # в нашем примере адреса "прыгают" по тем же сетёвкам, по которым ходит служебный VRRP-трафик, поэтому мы не указываем параметр interface
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

Шлюз с парой IP-адресов для LAN и WAN. В случае шлюза приватный и публичный IP друг без друга не могут и "прыгать" между узлами они будут вместе. Служебный VRRP-трафик в данном примере мы решили пустить через LAN-интерфейс, который мы определим с помощью метода NetworkAddress (считаем, что на каждом узле есть IP из этой подсети).

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
  - id: 4 # id "1", "2", "3" уже заняты в KeepalivedInstance "front" выше
    interface:
      detectionStrategy: NetworkAddress
      networkAddress: 192.168.42.0/24
    virtualIPAddresses:
    - address: 192.168.42.1/24
      # в данном случае мы уже определили локальную сеть выше и можем не определять интерфейс для этого IP, не указав параметр interface
    - address: 42.43.44.1/28
      interface:
        detectionStrategy: Name
        name: ens7 # на всех узлах интерфейс для публичных IP называется "ens7", воспользуемся этим
```
