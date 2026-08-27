---
title: Модуль managed-opensearch
permalink: ru/architecture/managed-services/managed-opensearch.html
lang: ru
search: managed-opensearch, opensearch
description: Архитектура модуля managed-opensearch в Deckhouse Kubernetes Platform.
---

Модуль [`managed-opensearch`](/modules/managed-opensearch/) управляет экземплярами [OpenSearch](https://github.com/opensearch-project/opensearch) в Deckhouse Kubernetes Platform (DKP). OpenSearch — это поисковая система и аналитический движок с открытым исходным кодом, предназначенный для работы с большими объёмами данных в реальном времени.
Модуль предоставляет:

* **Автоматическое развёртывание** — создаёт экземпляр OpenSearch при помощи простой YAML-конфигурации;
* **Управление конфигурацией** — отдельный ресурс [OpensearchClass](/modules/managed-opensearch/cr.html#opensearchclass) для шаблонизации создания сервиса и гибкой валидации пользовательских параметров;
* **Класс-ориентированное управление** — поддерживает sizing-политики и CEL-валидации;
* **Долговременное хранение** — поддерживает параметры PersistentVolumeClaim для данных;
* **Статус** — отображает текущее состояние развёрнутого экземпляра OpenSearch.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в документации модуля](/modules/managed-opensearch/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-opensearch`](/modules/managed-opensearch/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля managed-opensearch](../../images/architecture/managed-services/c4-l2-managed-opensearch.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. managed-opensearch-operator (Deployment) — оператор Kubernetes, состоящий из одного контейнера manager и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [Opensearch](/modules/managed-opensearch/cr.html#opensearch) во всех пользовательских неймспейсах. Ресурс Opensearch определяет настройки экземпляра OpenSearch;

   - создание и управление ресурсами StatefulSet, Service, Secret и PersistentVolumeClaim, относящимися к экземпляру OpenSearch.

1. managed-opensearch-webhook (Deployment) — компонент, состоящий из одного контейнера manager.

   Компонент выполняет валидацию и мутацию кастомных ресурсов [Opensearch](/modules/managed-opensearch/cr.html#opensearch) с помощью механизмов [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. d8ms-osch-\<INSTANCE_NAME> (StatefulSet) — компонент, состоящий из одного контейнера opensearch, который выполняет запуск и подготовку экземпляра OpenSearch. Создаётся компонентом managed-opensearch-operator.

## Взаимодействия модуля

Модуль взаимодействует с компонентом kube-apiserver:

- управляет кастомными ресурсами Opensearch и [Certificate](https://cert-manager.io/docs/usage/certificate/);
- следит за кастомными ресурсами OpensearchClass;
- управляет ресурсами StatefulSet, Service, Secret и PersistentVolumeClaim.

С модулем взаимодействуют следующие внешние компоненты:

1. kube-apiserver — обрабатывает запросы на валидацию и мутацию кастомных ресурсов Opensearch.

1. Пользовательские приложения — отправляют запросы к экземпляру OpenSearch.
