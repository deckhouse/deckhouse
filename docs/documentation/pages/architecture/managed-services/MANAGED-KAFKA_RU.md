---
title: Модуль managed-kafka
permalink: ru/architecture/managed-services/managed-kafka.html
lang: ru
search: managed-kafka, kafka
description: Архитектура модуля managed-kafka в Deckhouse Platform.
---

Модуль [`managed-kafka`](/modules/managed-kafka/) управляет инстансами [Apache Kafka](https://github.com/apache/kafka) в Deckhouse Platform (DP). Apache Kafka — это распределённая платформа потоковой передачи данных и брокер сообщений с открытым исходным кодом.
Модуль предоставляет:

* **Автоматическое развёртывание** — создаёт инстанс Kafka при помощи простой YAML-конфигурации;
* **Управление конфигурацией** — отдельный ресурс [KafkaClass](/modules/managed-kafka/cr.html#kafkaclass) для шаблонизации создания сервиса и гибкой валидации пользовательских параметров;
* **Долговременное хранение** — поддерживает параметры PersistentVolumeClaim для данных;
* **Статус** — отображает текущее состояние развёрнутого инстанса Kafka.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в документации модуля](/modules/managed-kafka/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`managed-kafka`](/modules/managed-kafka/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DP изображены на следующей диаграмме:

![Архитектура модуля managed-kafka](../../images/architecture/managed-services/c4-l2-managed-kafka.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Managed-kafka-operator** (Deployment) — оператор Kubernetes, состоящий из одного контейнера **manager** и выполняющий следующие операции:

   - согласование состояния кастомных ресурсов [Kafka](/modules/managed-kafka/cr.html#kafka) во всех пользовательских неймспейсах. Кастомные ресурсы Kafka и KafkaClass определяют настройки инстанса Kafka;

   - создание и управление кастомным ресурсом [Certificate](https://cert-manager.io/docs/usage/certificate/) и ресурсами StatefulSet, Service, Secret и PersistentVolumeClaim, относящимися к инстансу Kafka.

1. **Managed-kafka-webhook** (Deployment) — компонент, состоящий из одного контейнера **manager**.

   Компонент выполняет валидацию и мутацию кастомных ресурсов [Kafka](/modules/managed-kafka/cr.html#kafka) и [KafkaClass](/modules/managed-kafka/cr.html#kafkaclass) с помощью механизмов [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

1. **D8ms-kfk-\<INSTANCE_NAME>** (StatefulSet) — компонент, состоящий из одного контейнера **kafka** и отвечающий за запуск и работу инстанса Kafka. Создаётся компонентом managed-kafka-operator.

## Взаимодействия модуля

Модуль взаимодействует с компонентом **kube-apiserver**:

- управляет кастомными ресурсами Kafka и Certificate;
- следит за кастомными ресурсами KafkaClass;
- управляет ресурсами StatefulSet, Service, Secret и PersistentVolumeClaim.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — валидирует и мутирует кастомные ресурсы Kafka и KafkaClass.

1. **Пользовательские приложения** — отправляют запросы к инстансу Kafka.
