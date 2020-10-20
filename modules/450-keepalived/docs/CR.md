---
title: "Модуль keepalived: Custom Resources"
---

## KeepalivedInstance

Для настройки keepalived-кластеров объявлен кастомный ресурс — `KeepalivedInstance`. Один объект такого типа описывает отдельный keepalived-кластер.

* `metadata.name` — имя кластера, используется в названиях подов.
* `spec`:
    * `nodeSelector` — селектор для подов с keepalived.
        * Формат — [стандартный](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector) словарь лейблов и их значений. Поды инстанса унаследуют это поле как есть.
        * Обязательный параметр.
    * `tolerations` — толерейшны для подов с keepalived.
        * Формат — [стандартный](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) список толерейшнов. Поды инстанса унаследуют это поле как есть.
        * Необязательный параметр.
    * `vrrpInstances` — список инстансов VRRP внутри keepalived-кластера. По сути, список групп адресов, которые мигрируют между серверами одновременно и друг без друга не могут. Не путать понятия `vrrpInstance` и `KeepalivedInstance`, одно — это составная часть другого. Данный модуль настраивает VRRP-инстансы таким образом, чтобы все адреса (все группы) не собирались одновременно на одной ноде, а распределялись равномерно по всем серверам. Параметры одного vrrp-инстанса:
        * `id` — уникальный **в масштабах всего кластера** идентификатор VRRP-группы. Нельзя использовать одинаковый ID в разных инстансах KeepalivedInstance если у вас на это нет особой причины.
            * Формат — число от 1 до 255.
        * `interface` — определяет, как вычислить интерфейс для служебного VRRP-трафика на ноде:
            * Обязательный параметр.
            * `detectionStrategy` — одна из трёх возможных стратегий детекта:
                * `Name` — задать имя интерфейса явно, с помощью параметра `spec.vrrpInstances[].interface.name`. В этом случае все ноды должны иметь одинаковый интерфейс, который смотрит в нужную сеть (например, eth0).
                * `NetworkAddress` — найти на ноде интерфейс с ip из этой подсети и использовать его.
                * `DefaultRoute` — найти на ноде интерфейс, через который лежит дефолтный маршрут (в таблице 254 "main").
            * `name` — явное имя интерфейса для служебного VRRP-трафика в случае использования `detectionStrategy` = `Name`.
                * Обязательный параметр в случае использования `detectionStrategy` = `Name`.
            * `networkAddress` — интерфейс ноды с IP-адресом из этой подсети будет использован как служебный в случае использования `detectionStrategy` = `NetworkAddress`.
                * Формат — IP/Prefix, пример — 192.168.42.0/24.
                * Обязательный параметр в случае использования `detectionStrategy` = `NetworkAddress`.
        * `preempt` — возвращать ли IP на ноду, которая восстановилась после аварии. Если у вас один `vrrpInstance`, то разумнее не перекидывать IP лишний раз дабы не тревожить коннекты. Если групп много и трафик большой — то лучше вернуть дабы не допустить скопления всех групп на одной ноде.
            * По-умолчанию — `true`, то есть, IP вернётся ноду в случае если она встанет в строй.
        * `virtualIPAddresses` — список IP-адресов, которые **одновременно** будут "прыгать" между серверами:
            * `address` — собственно, один из адресов в группе.
                * Формат — IP/Prefix, пример — 192.168.42.15/32.
                * Обязательный параметр.
            * `interface` — аналогично `spec.vrrpInstances[].interface`, интерфейс для "приземления" IP-адреса на ноде.
                * Необязательный параметр. Если не указать — будет использован основной, служебный интерфейс, который задетектили в `spec.vrrpInstances[].interface`.
                * `detectionStrategy` — одна из трёх возможных стратегий детекта:
                    * `Name` — задать имя интерфейса явно, с помощью параметра `spec.vrrpInstances[].virtualIPAddresses[].interface.name`. В этом случае все ноды должны иметь одинаковый интерфейс, который смотрит в нужную сеть (например, eth0).
                    * `NetworkAddress` — найти на ноде интерфейс с ip из этой подсети и использовать его.
                    * `DefaultRoute` — найти на ноде интерфейс, через который лежит дефолтный маршрут (в таблице 254 "main").
                * `name` — явное имя интерфейса для виртуального IP в случае использования `virtualIPaddresses[].detectionStrategy` = `Name`.
                    * Обязательный параметр в случае использования `virtualIPaddresses[].detectionStrategy` = `Name`.
                * `networkAddress` — интерфейс ноды с IP-адресом из этой подсети будет использован интерфейс для данного виртуального IP в случае использования `detectionStrategy` = `NetworkAddress`.
                    * Формат — IP/Prefix, пример — 192.168.42.0/24.
                    * Обязательный параметр в случае использования `virtualIPaddresses[].detectionStrategy` = `NetworkAddress`.

## Примеры

### Три публичных IP-адреса

Три публичных IP-адреса на трёх фронтах. Каждый виртуальный IP-адрес вынесен в отдельную VRRP-группу, таким образом, каждый адрес "прыгает" независимо от других и если в кластере три ноды с лейблами `node-role/frontend: ""`, то каждый IP получит по своей MASTER-ноде.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front
spec:
  nodeSelector: # обязательно
    node-role/frontend: ""
  tolerations:  # опционально
  - key: dedicated
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

Шлюз с парой IP-адресов для LAN и WAN. В случае шлюза серый и белый IP друг без друга не могут и "прыгать" между нодами они будут вместе. Служебный VRRP-трафик в данном примере мы решили пустить через LAN-интерфейс, который мы задетектим с помощью метода NetworkAddress (считаем, что на каждой ноде есть IP из этой подсети).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: mygateway
spec:
  nodeSelector:
    node-role/mygateway: ""
  tolerations:
  - key: node-role/mygateway
    operator: Exists
  vrrpInstances:
  - id: 4 # id "1", "2", "3" уже заняты в KeepalivedInstance "front" выше
    interface:
      detectionStrategy: NetworkAddress
      networkAddress: 192.168.42.0/24
    virtualIPAddresses:
    - address: 192.168.42.1/24
      # в данном случае мы уже задетектили локалку выше и можем не детектить интерфейс для этого IP, не указав параметр interface
    - address: 42.43.44.1/28
      interface:
        detectionStrategy: Name
        name: ens7 # на всех нодах интерфейс для публичных IP называется "ens7", воспользуемся этим
```
