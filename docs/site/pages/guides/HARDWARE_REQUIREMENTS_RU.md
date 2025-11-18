---
title: Подбор ресурсов для кластера на bare metal
permalink: ru/guides/hardware-requirements.html
description: Аппаратные требования к узлам кластера под управлением Deckhouse Kubernetes Platform.
lang: ru
layout: sidebar-guides
---

Перед развёртыванием кластера под управлением Deckhouse Kubernetes Platform необходимо определиться с конфигурацией будущего кластера и выбрать параметры для будущих узлов кластера, такие как количество RAM, CPU и так далее.

## Планирование установки

Прежде чем приступить к развёртыванию кластера, необходимо провести планирование ресурсов, которые могут потребоваться для его работы. Для этого следует ответить на несколько вопросов:

* Какая нагрузка планируется на кластер?
* Требуется ли кластеру режим повышенной нагрузки?
* Требуется ли кластеру режим высокой доступности?
* Какие модули DKP планируется использовать?

Ответы на эти вопросы помогут определить необходимое количество узлов для развёртывания кластера. См. подробнее [в разделе «Сценарии развёртывания»](#сценарии-развертывания).

{% alert level="info" %}
Все описанное дальше применимо к установке Deckhouse Kubernetes Platform [с набором модулей Default](/products/kubernetes-platform/documentation/v1/admin/configuration/#наборы-модулей).
{% endalert %}

## Сценарии развёртывания

В разделе приведён **примерный расчёт ресурсов**, необходимых для кластера, в зависимости от предполагаемой нагрузки.

<table>
  <thead>
    <tr>
      <th>Конфигурация кластера</th>
      <th style="text-align: center;">Master-узлы</th>
      <th style="text-align: center;">Worker-узлы</th>
      <th style="text-align: center;">Frontend-узлы</th>
      <th style="text-align: center;">Системные узлы</th>
      <th style="text-align: center;">Узлы мониторинга</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Минимальная</td>
      <td style="text-align: center;">1</td>
      <td style="text-align: center;">не менее 1</td>
      <td style="text-align: center;">–</td>
      <td style="text-align: center;">–</td>
      <td style="text-align: center;">–</td>
    </tr>
    <tr>
      <td>Типовая</td>
      <td style="text-align: center;">3</td>
      <td style="text-align: center;">не менее 1</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">-</td>
    </tr>
    <tr>
      <td>Повышенная нагрузка</td>
      <td style="text-align: center;">3</td>
      <td style="text-align: center;">не менее 1</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">2</td>
    </tr>
  </tbody>
</table>

Типы узлов в таблице:

* **master-узлы** — узлы, управляющие кластером;
* **frontend-узлы** — узлы, балансирующие входящий трафик, на них работают Ingress-контроллеры;
* **узлы мониторинга** — служат для запуска Grafana, Prometheus и других компонентов мониторинга;
* **системные узлы** — предназначены для запуска модулей Deckhouse;
* **worker-узлы** — предназначены для запуска пользовательских приложений.

Подробнее с этими типами узлов можно ознакомиться [в секции «Особенности конфигурации»](https://deckhouse.ru/products/kubernetes-platform/guides/production.html#%D0%BE%D1%81%D0%BE%D0%B1%D0%B5%D0%BD%D0%BD%D0%BE%D1%81%D1%82%D0%B8-%D0%BA%D0%BE%D0%BD%D1%84%D0%B8%D0%B3%D1%83%D1%80%D0%B0%D1%86%D0%B8%D0%B8) раздела «Подготовка к production».

Приведённые в таблице конфигурации:

* **Минимальная** — кластер в такой конфигурации подходит для небольших проектов с невысокой нагрузкой и надёжностью. Характеристики worker-узла выбираются самостоятельно исходя из предполагаемой пользовательской нагрузки. Также следует учитывать, что в такой конфигурации некоторые из компонентов DKP также будут работать на worker-узле.
  > Использование такого кластера может быть небезопасно, так как в случае выхода из строя единственного master-узла пострадает весь кластер.
* **Типовая** – рекомендуемая конфигурация. Устойчива к отказам до двух мастер-узлов. Значительно повышает доступность сервисов.
* **Кластер с повышенной нагрузкой** — отличается от типовой конфигурации выделенными узлами мониторинга. Позволяет обеспечить высокий уровень наблюдаемости в кластере даже при высоких нагрузках.

## Выбор ресурсов для узлов

<table>
  <thead>
    <tr>
      <th>Уровень требований</th>
      <th>Тип узла</th>
      <th style="text-align: center;">CPU (шт.)</th>
      <th style="text-align: center;">RAM (ГБ)</th>
      <th style="text-align: center;">Объем диска (ГБ)</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td rowspan="6" style="width: 45%;">
        <b>Минимальные</b><br><br>
        <i>Работа кластера на узлах с минимальными требованиями сильно зависит от набора включённых модулей DKP.<br>
        При большом количестве включённых модулей ресурсы узлов лучше увеличить.<br><br>
        </i>
      </td>
      <td>Master-узел</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Worker-узел</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Frontend-узел</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">50</td>
    </tr>
    <tr>
      <td>Узел мониторинга</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>Системный узел</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>Системный узел <i>(если нет выделенных узлов мониторинга</i>)</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;"><a href="#storage">60 / 160*</a></td>
    </tr>
    <tr>
      <td rowspan="6" style="width: 45%;">
        <b>Production</b><br><br>
      </td>
      <td>Master-узел</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">16</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Worker-узел</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">12</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Frontend-узел</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">50</td>
    </tr>
    <tr>
      <td>Узел мониторинга</td>
      <td style="text-align: center;">6</td>
      <td style="text-align: center;">12</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>Системный узел</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>Системный узел <i>(если нет выделенных узлов мониторинга</i>)</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">16</td>
      <td style="text-align: center;"><a href="#storage">60 / 160*</a></td>
    </tr>
    <tr>
      <td style="width: 45%;">
        <b>Кластер с одним-единственным master-узлом</b>
      </td>
      <td>Master-узел</td>
      <td style="text-align: center;">6</td>
      <td style="text-align: center;">12</td>
      <td style="text-align: center;">160</td>
    </tr>
  </tbody>
</table>

{% alert %}
* <span id="storage"></span>Дисковое пространство PVC для системных компонентов: если для хранения системных PVC (модулей prometheus, upmeter и других) будет использоваться локальное дисковое пространство узла, то необходимо дополнительно выделить >= 100 ГБ.
* Характеристики worker-узлов во многом зависят от характера запускаемой на узле (узлах) нагрузки, в таблице указаны минимальные требования. Под системные сервисы (kubelet) и системные поды на worker-узлах требуется заложить как минимум 1 CPU и 2 ГБ памяти.
* Для всех узлов следует выделять быстрые диски (400+ IOPS).
{% endalert %}

### Кластер с одним-единственным master-узлом

{% alert level="warning" %}
У такого кластера отсутствует отказоустойчивость. Его использование не рекомендуется в production-окружениях.
{% endalert %}

В некоторых случаях может быть достаточно всего одного-единственного узла, который будет выполнять все описанные выше роли узлов в одиночку. Например, это может быть полезно в ознакомительных целях или для каких-то совсем простых задач, не требовательных к ресурсам.

В [«Быстром старте»](/products/kubernetes-platform/gs/bm/step5.html) есть инструкции по развёртыванию кластера на единственном master-узле. После снятия taint с узла на нём будут запущены все компоненты кластера, входящие в выбранный набор модулей (по умолчанию — [Default](/modules/deckhouse/configuration.html#parameters-bundle)). Для успешной работы кластера в таком режиме потребуются 16 CPU, 32 ГБ RAM и 60 ГБ дискового пространства на быстром диске (400+ IOPS). Эта конфигурация позволит запускать некоторую полезную нагрузку.

В такой конфигурации при нагрузке в 2500 RPS на условное веб-приложение (статическая страница Nginx) из 30 подов и входящем трафике в 24 Мбит/с:

- нагрузка на CPU суммарно будет повышаться до ~60%;
- значения RAM и диска не возрастают, но в реальности будут зависеть от количества метрик, собираемых с приложений, и характера обработки полезной нагрузки.

{% alert level="info" %}
Рекомендуется провести нагрузочное тестирование вашего приложения и с учетом этого скорректировать мощности сервера.
{% endalert %}

### Примеры конфигураций

Для развертывания Deckhouse Kubernetes Platform в редакции [Enterprise Edition](../pricing/#revisions) с набором модулей [Default](..//documentation/v1/modules/deckhouse/configuration.html#parameters-bundle) необходима следующая конфигурация узлов:

* **master-узлы** — 1 шт, 4 CPU, 8 ГБ RAM;
* **frontend-узлы** — 1 шт, 2 CPU, 4 ГБ RAM;
* **системные узлы** — 1 шт, 8 CPU, 16 ГБ RAM;
* **worker-узлы** — 1 шт, 4 CPU, 8 ГБ RAM.

{% alert level="info" %}
При необходимости DKP в такой же конфигурации можно запустить и на одном узле с 16 CPU, 32 ГБ RAM для виртуальной машины или 10 CPU, 24 ГБ RAM для bare-metal-сервера.
{% endalert %}

## Требования к аппаратным характеристикам узлов

Машины, предназначенные стать узлами будущего кластера, должны соответствовать следующим требованиям:

* **Архитектура ЦП** — на всех узлах должна использоваться архитектура ЦП `x86_64`.
* **Однотипные узлы** — все узлы должны иметь одинаковую конфигурацию для каждого типа узлов. Узлы должны быть одной марки и модели с одинаковой конфигурацией ЦП, памяти и хранилища.
* **Сетевые интерфейсы** — каждый узел должен иметь по крайней мере один сетевой интерфейс для маршрутизируемой сети.

## Требования к сети между узлами

* Узлы должны иметь сетевой доступ друг к другу. Между узлами должны соблюдаться [сетевые политики](../documentation/v1/network_security_setup.html).
* Требований к MTU нет.
* У каждого узла должен быть постоянный IP-адрес. В случае использования DHCP-сервера для распределения IP-адресов по узлам необходимо настроить в нём чёткое соответствие выдаваемых адресов каждому узлу. Смена IP-адреса узлов нежелательна.
* Доступ к внешним для кластера источникам времени по NTP должен быть открыт как минимум для master-узлов. Узлы кластера синхронизируют время с master-узлами, но могут синхронизироваться также и с другими серверами времени (параметр [ntpServers](../documentation/v1/modules/chrony/configuration.html#parameters-ntpservers)).

## Сообщество

{% alert %}
Следите за новостями проекта [в Telegram](https://t.me/deckhouse_ru).
{% endalert %}

Вступите [в сообщество](https://deckhouse.ru/community/about.html), чтобы быть в курсе важных изменений и новостей. Вы сможете общаться с людьми, занятыми общим делом. Это позволит избежать многих типичных проблем.

Команда Deckhouse знает, каких усилий требует организация работы production-кластера в Kubernetes. Мы будем рады, если Deckhouse позволит вам реализовать задуманное. Поделитесь своим опытом и вдохновите других на переход в Kubernetes.
