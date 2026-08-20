---
title: Модуль managed-valkey
permalink: ru/architecture/managed-services/managed-valkey.html
lang: ru
search: managed-valkey, valkey
description: Архитектура модуля managed-valkey в Deckhouse Kubernetes Platform.
---

Модуль [`managed-valkey`](/modules/managed-valkey/) управляет кластерами [Valkey](https://github.com/valkey-io/valkey) (Redis-совместимое хранилище данных в оперативной памяти) в Deckhouse Kubernetes Platform (DKP). Он предоставляет:

- **Автоматическое развёртывание** — разворачивает инстанс Valkey при помощи простой YAML-конфигурации;
- **Standalone** — поддерживает установку одиночного инстанса;
- **Persistent Storage** — позволяет сконфигурировать разные варианты хранения данных `AOF`, `RDB`, `AOF+RDB`;
- **Управление конфигурацией** — отдельный ресурс [ValkeyClass](/modules/managed-valkey/cr.html#valkeyclass) для шаблонизации подхода к созданию сервиса с возможностью гибко валидировать пользовательские параметры;
- **Статус** — информативный набор состояний для отслеживания развёрнутого Valkey.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/managed-valkey/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-valkey`](/modules/managed-valkey/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображена на следующей диаграмме:

![Архитектура модуля managed-valkey](../../images/architecture/managed-services/c4-l2-managed-valkey.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Managed-valkey-operator** (Deployment) — оператор Kubernetes, состоящий из одного контейнера **manager** и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [Valkey](/modules/managed-valkey/cr.html#valkey) во всех пользовательских неймспейсах. Ресурс Valkey определяет настройки инстанса Valkey;

   - создание и управление ресурсами StatefulSet, Secret, ConfigMap и PersistentVolumeClaim, относящимися к инстансу Valkey.

1. **Managed-valkey-webhook** (Deployment) — компонент состоит из одного контейнера **manager**.

   Managed-valkey-webhook выполняет валидацию и мутацию кастомных ресурсов [Valkey](/modules/managed-valkey/cr.html#valkey), а также мутацию кастомных ресурсов [ValkeyClass](/modules/managed-valkey/cr.html#valkeyclass) с помощью механизма [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. **D8ms-valkey-\<INSTANCE_NAME>** (StatefulSet) — компонент выполняет запуск и подготовку инстанса Valkey. Компонент создаётся компонентом managed-valkey-operator.

   Состоит из двух контейнеров:

   - **valkey** — является [Open Source-проектом](https://github.com/valkey-io/valkey);
   - **agent** — сайдкар-контейнер, выполняющий настройку основного контейнера в соответствии с параметрами в ресурсе Valkey.

## Взаимодействия модуля

Модуль взаимодействует с компонентом **kube-apiserver**:

- управляет кастомными ресурсами Valkey, ValkeyClass и [Certificate](https://cert-manager.io/docs/usage/certificate/);
- управляет ресурсами StatefulSet, Secret, ConfigMap и PersistentVolumeClaim.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — отправляет запросы на валидацию и мутацию кастомных ресурсов Valkey.

1. **Prometheus-main** — собирает метрики компонентов managed-valkey-operator и managed-valkey-webhook.

1. **Пользовательские приложения** — отправляют запросы к инстансу Valkey.
