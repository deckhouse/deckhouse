---
title: Модуль managed-cassandra
permalink: ru/architecture/managed-services/managed-cassandra.html
lang: ru
search: managed-cassandra, cassandra
description: Архитектура модуля managed-cassandra в Deckhouse Kubernetes Platform.
---

Модуль [`managed-cassandra`](/modules/managed-cassandra/) управляет экземплярами распределённой системы управления базами данных класса NoSQL с открытым исходным кодом [Apache Cassandra](https://github.com/apache/cassandra) в Deckhouse Kubernetes Platform (DKP). Он предоставляет:

* **Автоматическое развёртывание** — создаёт экземпляр Cassandra при помощи простой YAML-конфигурации;
* **Standalone** — поддерживает установку одиночного экземпляра;
* **Управление конфигурацией** — отдельный ресурс [CassandraClass](/modules/managed-cassandra/cr.html#cassandraclass) для шаблонизации создания сервиса и гибкой валидации пользовательских параметров;
* **Статус** — отображает текущее состояние развёрнутого экземпляра Cassandra.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в документации модуля](/modules/managed-cassandra/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-cassandra`](/modules/managed-cassandra/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля managed-cassandra](../../images/architecture/managed-services/c4-l2-managed-cassandra.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. managed-cassandra-operator (Deployment) — оператор Kubernetes, состоящий из одного контейнера manager и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [Cassandra](/modules/managed-cassandra/cr.html#cassandra) во всех пользовательских неймспейсах. Ресурс Cassandra определяет настройки экземпляра Cassandra;

   - создание и управление ресурсами StatefulSet, Service, Secret и PersistentVolumeClaim, относящимися к экземпляру Cassandra;

   - валидация и мутация кастомных ресурсов Cassandra с помощью механизмов [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. d8ms-cas-\<INSTANCE_NAME> (StatefulSet) — компонент, состоящий из одного контейнера cassandra, который выполняет запуск экземпляра Cassandra.

## Взаимодействия модуля

Модуль взаимодействует с компонентом kube-apiserver:

- управляет кастомными ресурсами Cassandra;
- следит за кастомными ресурсами CassandraClass;
- управляет ресурсами StatefulSet, Service, Secret и PersistentVolumeClaim.

С модулем взаимодействуют следующие внешние компоненты:

1. kube-apiserver — обрабатывает запросы на валидацию и мутацию кастомных ресурсов Cassandra.

1. Пользовательские приложения — отправляют запросы к экземпляру Cassandra.
