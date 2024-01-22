---
title: "Модуль keepalived: примеры"
---

## Три публичных IP-адреса

Имеются три публичных IP-адреса, каждый из которых привязан к отдельному front-серверу. Каждый из виртуальных IP-адресов входит в отдельную группу VRRP, поэтому каждый адрес переключается независимо от других. Если в кластере присутствуют три узла с меткой `node-role.deckhouse.io/frontend: ""`, то каждый IP будет привязан к своему главному серверу.

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

Имеется шлюз с двумя IP-адресами: один для внутренней (LAN) и один для внешней (WAN) сети. Эти два IP работают в паре и вместе переключаются между узлами. Внутренний интерфейс (LAN) используется для служебного трафика VRRP (трафик, используемый для управления группой VRRP). Этот интерфейс определяется с помощью функции `NetworkAddress` c предположением, что каждый узел имеет IP-адрес из одной и той же подсети.

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
