---
title: "Подключение дополнительных сетей к подам"
permalink: ru/user/network/sdn/pod-connecting-dedicated-networks.html
lang: ru
---

Чтобы подключить дополнительные сети к поду используйте аннотацию пода, в которой укажите параметры подключаемых дополнительных сетей:

```yaml
network.deckhouse.io/networks-spec: |
  [
    {
      "type": "Network", # Подключение сети проекта my-network.
      "name": "my-network",
      "ifName": "veth_mynet",    # Имя TAP-интерфейса внутри пода (опционально).
      "mac": "aa:bb:cc:dd:ee:ff" # MAC-адрес, который следует назначить TAP-интерфейсу (опционально).
    },
    {
      "type": "ClusterNetwork", # Подключение общедоступной сети my-cluster-network.
      "name": "my-cluster-network",
      "ifName": "veth_public",
    }
  ]
```
