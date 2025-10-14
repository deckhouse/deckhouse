---
title: "Балансировка средствами MetalLB"
permalink: ru/virtualization-platform/documentation/admin/platform-management/network/ingress/nlb/metallb.html
lang: ru
---

Модуль [`metallb`](/modules/metallb/) реализует поддержку сервисов типа LoadBalancer в кластерах Deckhouse Virtualization Platform (DVP).
Он подходит как для bare-metal-кластеров, так и для облачных, в которых недоступны встроенные балансировщики от провайдера.

<!-- перенесено с минимальными изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/metallb/ -->

Поддерживаются два режима работы:

- **Layer 2** — усовершенствованный (относительно стандартного режима L2 в MetalLB) механизм балансировки, который позволяет использовать несколько публичных адресов для сервисов.
- **BGP** — полностью основан на решении [MetalLB](https://metallb.universe.tf/) и доступен только в DVP Enterprise Edition.

## Режим Layer 2

### Принцип работы

В режиме Layer 2 один или несколько узлов кластера принимают трафик к сервису из публичной сети. Сетевой уровень воспринимает это так, как будто у каждого из этих узлов назначено несколько IP-адресов на сетевом интерфейсе.
Технически это реализуется следующим образом: модуль отвечает на ARP-запросы (для IPv4) и NDP-запросы (для IPv6).

Главное преимущество режима Layer 2 — универсальность: он работает в любой Ethernet-сети и не требует специализированного оборудования.

### Преимущества модуля перед классическим MetalLB

В классическом MetalLB (режим L2) при создании сервиса с типом LoadBalancer балансировка работает за счёт того, что один из узлов кластера имитирует ARP-ответы от публичного IP. Это означает:

- Одновременно только один узел обслуживает весь входящий трафик для данного IP.
- Узел, выбранный в качестве лидера, становится «узким местом» без возможности горизонтального масштабирования.
- При отказе узла все активные соединения обрываются во время переключения на новый узел.

<div data-presentation="/presentations/metallb/basics_metallb_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1cs1uKeX53DB973EMtLFcc8UQ8BFCW6FY2vmEWua1tu8/ --->

Модуль `metallb` устраняет эти ограничения. Он предоставляет ресурс MetalLoadBalancerClass, который позволяет:

- связать группу узлов с пулом IP-адресов с помощью `nodeSelector`;
- создать стандартный объект Service с типом LoadBalancer и указать имя нужного MetalLoadBalancerClass;
- через аннотацию определить количество IP-адресов для L2-анонсирования.

<div data-presentation="/presentations/metallb/basics_metallb_l2balancer_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1jDqC4Bhg5NMLZWaFM32bIAzqpkUo0hOkAaRzC0yKRxE/ --->

Таким образом:

- Приложение получает сразу несколько публичных IP. Их нужно указать как A-записи в DNS.
- Для масштабирования достаточно добавить балансировочные узлы; связанные с ними объекты Service создадутся автоматически — потребуется лишь добавить их в список A-записей прикладного домена.
- При выходе одного из балансировщиков из строя трафик переключится только частично, без полного разрыва соединений.

#### Сравнение поведения

| Характеристика                         | Классический MetalLB (L2) | Новый модуль с MetalLoadBalancerClass |
|----------------------------------------|----------------------------|----------------------------------------|
| Обработка трафика                      | Один узел (лидер)          | Несколько узлов                        |
| Масштабируемость                       | Нет                        | Да                                     |
| Устойчивость к сбоям                   | Обрыв всех соединений      | Часть трафика переключается плавно     |
| Количество публичных IP                | Один                       | Несколько (настраивается)              |
| DNS-настройка                          | Одна A-запись              | Несколько A-записей                    |

<!-- перенесено с изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/metallb/examples.html#%D0%BF%D1%80%D0%B8%D0%BC%D0%B5%D1%80-%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D1%8F-metallb-%D0%B2-%D1%80%D0%B5%D0%B6%D0%B8%D0%BC%D0%B5-l2-loadbalancer-->

### Пример использования MetalLB в режиме L2 LoadBalancer

1. Включите модуль `metallb`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Подготовьте приложение, которое хотите опубликовать:

   ```shell
   d8 k create deploy nginx --image=nginx
   ```

1. Создайте ресурс MetalLoadBalancerClass:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # Селектор узлов-балансировщиков.
     type: L2
   ```

1. Создайте ресурс Service с аннотацией и именем MetalLoadBalancerClass:

   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: nginx-deployment
     annotations:
       network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
   spec:
     type: LoadBalancer
     loadBalancerClass: ingress # Имя MetalLoadBalancerClass.
     ports:
     - port: 8000
       protocol: TCP
       targetPort: 80
     selector:
       app: nginx
   ```

В результате, созданному сервису с типом LoadBalancer будут присвоены адреса в заданном количестве:

```shell
d8 k get svc
```

Пример вывода:

```console
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)        AGE
nginx-deployment       LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30544/TCP   11s
```

Полученные `EXTERNAL-IP` можно прописывать в качестве A-записей для прикладного домена:

```shell
curl -s -o /dev/null -w "%{http_code}" 192.168.2.100:8000
curl -s -o /dev/null -w "%{http_code}" 192.168.2.101:8000
curl -s -o /dev/null -w "%{http_code}" 192.168.2.102:8000
```

Пример вывода:

```console
200
```

## Режим BGP

{% alert level="info" %}
Доступен только в DVP Enterprise Edition.
{% endalert %}

`metallb` в режиме BGP используется для предоставления сервисов типа LoadBalancer в кластерах, развёрнутых на физической инфраструктуре.
IP-адреса сервисов анонсируются напрямую в маршрутизаторы (или Top-of-Rack-коммутаторы) с помощью протокола BGP.

### Принцип работы

#### Конфигурация

- Определяется пул IP-адресов, доступных для назначения сервисам.
- Указываются параметры BGP-сессий: AS (Autonomous System) number кластера, IP-адреса маршрутизаторов (пиров), AS number пиров, пароли для аутентификации (при необходимости).
- Для каждого пула IP-адресов могут быть заданы специфические параметры анонсирования, например, community strings.

#### Установка BGP-сессий

- На каждом узле кластера, где запущен `metallb`, компонент `speaker` устанавливает BGP-сессии с указанными маршрутизаторами.
- Выполняется обмен маршрутной информацией между `metallb` и маршрутизаторами.

#### Назначение IP-адресов сервисам

- При создании объекта Service типа LoadBalancer, `metallb` выбирает свободный IP-адрес из сконфигурированного пула и назначает его сервису.
- Компонент `controller` отслеживает изменения в сервисах и управляет назначениями IP-адресов.

#### Анонсирование IP-адресов

- После назначения IP-адреса компонент `speaker` на одном из узлов (лидере для данного сервиса) начинает анонсировать его через установленные BGP-сессии.
- Маршрутизаторы получают анонс и обновляют таблицы маршрутизации, направляя трафик на соответствующий узел.

#### Распределение трафика

- Маршрутизаторы используют протоколы Equal-Cost Multi-Path (ECMP) или другие алгоритмы балансировки для распределения трафика между узлами, анонсирующими один и тот же IP-адрес сервиса.
- После доставки на узел входящий трафик перенаправляется на поды сервиса с помощью механизмов используемого CNI (iptables/IPVS, eBPF-программы и т.д.).

### Преимущества использования BGP

- Протокол BGP поддерживается большинством сетевого оборудования.
- Сеть может включать несколько маршрутизаторов и большое число узлов.
- Балансировка трафика через ECMP.
- При выходе из строя узла, анонсирующего IP-адрес, маршрутизаторы автоматически перенаправляют трафик на другие узлы с этим же IP.

### Недостатки использования BGP

- Более высокая сложность настройки, чем настройка анонсов ARP/GARP (в режиме Layer 2).
- Маршрутизаторы должны поддерживать BGP и ECMP.

<!-- перенесено с минимальными изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/metallb/examples.html#%D0%BF%D1%80%D0%B8%D0%BC%D0%B5%D1%80-%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D1%8F-metallb-%D0%B2-%D1%80%D0%B5%D0%B6%D0%B8%D0%BC%D0%B5-bgp-loadbalancer -->

### Пример использования MetalLB в режиме BGP LoadBalancer

1. Включите модуль `metallb` и настройте все необходимые параметры:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     settings:
       addressPools:
       - addresses:
         - 192.168.219.100-192.168.219.200
         name: mypool
         protocol: bgp
       bgpPeers:
       - hold-time: 3s
         my-asn: 64600
         peer-address: 172.18.18.10
         peer-asn: 64601
       speaker:
         nodeSelector:
           node-role.deckhouse.io/metallb: ""
     version: 2
   ```

1. Настройте BGP-пиринг на сетевом оборудовании.
