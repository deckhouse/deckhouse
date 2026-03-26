---
title: Модуль terraform-manager
permalink: ru/architecture/cluster-and-infrastructure/infrastructure/terraform-manager.html
lang: ru
search: terraform manager, terraform
description: Архитектура модуля terraform-manager в Deckhouse Kubernetes Platform для управления состоянием Terraform и инфраструктурными ресурсами кластера.
---

Модуль `terraform-manager` предоставляет инструменты для работы с состоянием Terraform в кластере DKP.

Подробнее с настройками модуля можно ознакомиться в [соответствующем разделе документации](/modules/terraform-manager/configuration.html).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`terraform-manager`](/modules/terraform-manager/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля terraform-manager](../../../../images/architecture/cluster-and-infrastructure/c4-l2-terraform-manager.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Terraform-auto-converger** — периодически (по умолчанию раз в час) проверяет состояние Terraform и применяет недеструктивные изменения к ресурсам инфраструктуры.

   Компонент работает только с базовой инфраструктурой кластера. Узлы кластера автоматически к требуемому состоянию не приводятся. Периодичность проверки задается параметром [`autoConvergerPeriod`](/modules/terraform-manager/configuration.html#parameters-autoconvergerperiod).

   Состоит из следующих контейнеров:

   * **to-tofu-migrator** — init-контейнер для миграции состояния Terraform в OpenTofu. В контейнере запускается утилита [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) с командой `converge-migration`;
   * **converger** — основной контейнер, в котором запускается утилита [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) с командой `converge-periodical`;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера converger. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

2. **Terraform-state-exporter** — проверяет состояние Terraform и экспортирует связанные с ним метрики.

   Состоит из следующих контейнеров:

   * **exporter** — основной контейнер, в котором запускается утилита [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) с командой `terraform converge-exporter`;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контейнера exporter.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   * чтение и запись секрета с состоянием Terraform;
   * авторизация запросов на получение метрик.

2. **Облачная инфраструктура** (или система виртуализации) — управляет базовыми инфраструктурными ресурсами и приводит их к желаемому состоянию.

С модулем взаимодействуют следующие внешние компоненты:

* **prometheus-main** — сбор метрик terraform-auto-converger и terraform-state-exporter.
