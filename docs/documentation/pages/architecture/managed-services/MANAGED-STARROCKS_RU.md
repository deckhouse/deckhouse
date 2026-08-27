---
title: Модуль managed-starrocks
permalink: ru/architecture/managed-services/managed-starrocks.html
lang: ru
search: managed-starrocks, starrocks
description: Архитектура модуля managed-starrocks в Deckhouse Kubernetes Platform.
---

Модуль [`managed-starrocks`](/modules/managed-starrocks/) управляет экземплярами [StarRocks](https://github.com/starrocks/starrocks) в Deckhouse Kubernetes Platform (DKP). StarRocks — высокопроизводительная аналитическая СУБД (OLAP) для аналитики в реальном времени, хранилищ данных и BI-нагрузок.

Модуль предоставляет:

* **Автоматическое развёртывание** — создаёт экземпляр StarRocks при помощи простой YAML-конфигурации;
* **Управление конфигурацией** — отдельный ресурс [StarrocksClass](/modules/managed-starrocks/cr.html#starrocksclass) для шаблонизации создания сервиса и гибкой валидации пользовательских параметров;
* **Настройка под аналитику** — поддерживает указание параметров загрузки данных (`streamingLoadMaxMb`, `loadProcessMaxMemoryLimitPercent`) и управления каталогом (`catalogTrashExpireSecond`);
* **Статус** — отображает текущее состояние развёрнутого экземпляра StarRocks.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в документации модуля](/modules/managed-starrocks/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-starrocks`](/modules/managed-starrocks/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля managed-starrocks](../../images/architecture/managed-services/c4-l2-managed-starrocks.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. managed-starrocks-operator (Deployment) — оператор Kubernetes, состоящий из одного контейнера manager и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [Starrocks](/modules/managed-starrocks/cr.html#starrocks) во всех пользовательских неймспейсах. Ресурс Starrocks определяет настройки экземпляра StarRocks;

   - создание и управление ресурсами StatefulSet, Service, Secret, ConfigMap и PersistentVolumeClaim, относящимися к экземпляру StarRocks.

1. managed-starrocks-webhook (Deployment) — компонент, состоящий из одного контейнера manager.

   Компонент выполняет валидацию и мутацию кастомных ресурсов [Starrocks](/modules/managed-starrocks/cr.html#starrocks), а также мутацию кастомного ресурса [StarrocksClass](/modules/managed-starrocks/cr.html#starrocksclass) с помощью механизмов [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. d8ms-sr-\<INSTANCE_NAME> (StatefulSet) — компонент выполняет запуск и подготовку экземпляра StarRocks. Создаётся компонентом managed-starrocks-operator.

   Состоит из следующих контейнеров:

   - **agent** — сайдкар-контейнер, который отслеживает и обновляет статус TLS-сертификата, а также выполняет настройку экземпляра;
   - **starrocks** — основной контейнер.

## Взаимодействия модуля

Модуль взаимодействует с компонентом kube-apiserver:

- управляет кастомными ресурсами Starrocks и [Certificate](https://cert-manager.io/docs/usage/certificate/);
- следит за кастомными ресурсами StarrocksClass;
- управляет ресурсами StatefulSet, Service, Secret, ConfigMap и PersistentVolumeClaim.

С модулем взаимодействуют следующие внешние компоненты:

1. kube-apiserver — обрабатывает запросы на валидацию и мутацию кастомных ресурсов Starrocks и StarrocksClass.

1. Пользовательские приложения — отправляют запросы к экземпляру StarRocks.
