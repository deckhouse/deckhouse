---
title: Модуль managed-hive-metastore
permalink: ru/architecture/managed-services/managed-hive-metastore.html
lang: ru
search: managed-hive-metastore, hive-metastore
description: Архитектура модуля managed-hive-metastore в Deckhouse Platform.
---

Модуль [`managed-hive-metastore`](/modules/managed-hive-metastore/) управляет экземплярами централизованного хранилища метаданных в экосистеме больших данных [Hive Metastore (HMS)](https://github.com/apache/hive) в Deckhouse Platform (DP). Он предоставляет:

* **Автоматическое развёртывание** — создаёт экземпляр HMS при помощи простой YAML-конфигурации;
* **Standalone** — поддерживает установку одиночного экземпляра;
* **Управление конфигурацией** — отдельный ресурс [HiveMetastoreClass](/modules/managed-hive-metastore/cr.html#hive-metastoreclass) для шаблонизации создания сервиса и гибкой валидации пользовательских параметров;
* **Статус** — отображает текущее состояние развёрнутого экземпляра HMS.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в документации модуля](/modules/managed-hive-metastore/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-hive-metastore`](/modules/managed-hive-metastore/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DP изображены на следующей диаграмме:

![Архитектура модуля managed-hive-metastore](../../images/architecture/managed-services/c4-l2-managed-hive-metastore.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Managed-hive-metastore-operator** (Deployment) — оператор Kubernetes, состоящий из одного контейнера **manager** и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [HiveMetastore](/modules/managed-hive-metastore/cr.html#hive-metastore) во всех пользовательских неймспейсах. Кастомные ресурсы HiveMetastore и HiveMetastoreClass определяют настройки экземпляра HMS;

   - создание и управление кастомным ресурсом [Certificate](https://cert-manager.io/docs/usage/certificate/) и ресурсами Deployment, Service и Secret, относящимися к экземпляру HMS;

   - валидация и мутация кастомных ресурсов HiveMetastore с помощью механизмов [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. **D8ms-hms-\<INSTANCE_NAME>** (Deployment) — компонент, отвечающий за запуск и работу экземпляра HMS.

   Состоит из следующих контейнеров:

   - **truststore-prepare** — init-контейнер, выполняющий инициализацию хранилища доверенных сертификатов;
   - **agent** — сайдкар-контейнер, который отслеживает и обновляет статус TLS-сертификата;
   - **hivemetastore** — основной контейнер.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Экземпляр PostgreSQL** — обрабатывает метаданные на сервере базы данных.

1. **S3 (объектное хранилище)** — обрабатывает хранящиеся данные.

1. **Kube-apiserver**:

   - управляет кастомными ресурсами HiveMetastore и Certificate;
   - следит за кастомными ресурсами HiveMetastoreClass;
   - управляет ресурсами Deployment, Service и Secret.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — валидирует и мутирует кастомные ресурсы HiveMetastore.

1. **Пользовательские приложения** — отправляют запросы к экземпляру HMS.
