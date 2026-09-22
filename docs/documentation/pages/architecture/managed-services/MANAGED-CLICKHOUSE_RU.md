---
title: Модуль managed-clickhouse
permalink: ru/architecture/managed-services/managed-clickhouse.html
lang: ru
search: managed-clickhouse, clickhouse
description: Архитектура модуля managed-clickhouse в Deckhouse Platform.
---

Модуль [`managed-clickhouse`](/modules/managed-clickhouse/) управляет инстансами [ClickHouse](https://github.com/clickhouse/clickhouse) в Deckhouse Platform (DP). ClickHouse — высокопроизводительная колоночная система управления базами данных (СУБД) с открытым исходным кодом, предназначенная для онлайн-аналитической обработки (OLAP) больших объёмов данных в реальном времени.
Модуль предоставляет:

* **Автоматическое развёртывание** — создаёт инстанс ClickHouse при помощи простой YAML-конфигурации;
* **Standalone** — поддерживает установку одиночного инстанса;
* **Управление конфигурацией** — отдельный ресурс [ClickhouseClass](/modules/managed-clickhouse/cr.html#clickhouseclass) для шаблонизации создания сервиса и гибкой валидации пользовательских параметров;
* **Статус** — отображает текущее состояние развёрнутого инстанса ClickHouse.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в документации модуля](/modules/managed-clickhouse/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-clickhouse`](/modules/managed-clickhouse/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DP изображены на следующей диаграмме:

![Архитектура модуля managed-clickhouse](../../images/architecture/managed-services/c4-l2-managed-clickhouse.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Managed-clickhouse-operator** (Deployment) — оператор Kubernetes, состоящий из одного контейнера **manager** и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [Clickhouse](/modules/managed-clickhouse/cr.html#clickhouse) во всех пользовательских неймспейсах. Кастомные ресурсы Clickhouse и ClickhouseClass определяют настройки инстанса ClickHouse;

   - создание и управление кастомным ресурсом [Certificate](https://cert-manager.io/docs/usage/certificate/) и ресурсами StatefulSet, Secret, Service, ConfigMap и PersistentVolumeClaim, относящимися к инстансу ClickHouse;

   - валидация и мутация кастомных ресурсов Clickhouse с помощью механизмов [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. **D8ms-ch-\<INSTANCE_NAME>** (StatefulSet) — компонент, состоящий из одного контейнера **clickhouse** и отвечающий за запуск и работу инстанса ClickHouse. Создаётся компонентом managed-clickhouse-operator для каждого кастомного ресурса Clickhouse.

## Взаимодействия модуля

Модуль взаимодействует с компонентом **kube-apiserver**:

- управляет кастомными ресурсами Clickhouse и Certificate;
- следит за кастомными ресурсами ClickhouseClass;
- управляет ресурсами StatefulSet, Secret, Service, ConfigMap и PersistentVolumeClaim.

С модулем взаимодействуют следующие внешние компоненты:

1. **kube-apiserver** — валидирует и мутирует кастомные ресурсы Clickhouse.

1. **Пользовательские приложения** — отправляют запросы к инстансу ClickHouse.
