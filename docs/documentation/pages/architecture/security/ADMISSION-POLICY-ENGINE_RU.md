---
title: Модуль admission-policy-engine
permalink: ru/architecture/security/admission-policy-engine.html
lang: ru
search: admission-policy-engine, pod security, gatekeeper
description: Архитектура модуля admission-policy-engine в Deckhouse Kubernetes Platform.
---

Модуль `admission-policy-engine` предназначен для обработки политик безопасности в кластере согласно [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

Подробнее с описанием модуля можно ознакомиться [в разделе документации модуля](/modules/admission-policy-engine/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`admission-policy-engine`](/modules/admission-policy-engine/) на уровне 2 модели C4 и его взаимодействия с другими компонентами Deckhouse Kubernetes Platform (DKP) изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля admission-policy-engine](../../../images/architecture/security/c4-l2-admission-policy-engine.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Gatekeeper-controller-manager** — это контроллер ([Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/)), который проверяет создаваемые ресурсы Kubernetes на соответствие правилам безопасности.

   Правила безопасности задаются с помощью кастомных ресурсов ConstraintTemplate и `constraints.gatekeeper.sh/*`. ConstraintTemplate описывает новые типы политик, на основании которых создаются конкретные политики безопасности для проверки ресурсов.

   Так же **gatekeeper-controller-manager** выполняет мутацию ресурсов Kubernetes на основе следующих кастомных ресурсов Gatekeeper:

   * AssignMetadata — описывает правила изменения в секции `Metadata`;
   * Assign — описывает правила изменения полей, за пределом секции `Metadata`;
   * ModifySet — описывает правила добавления или удаления элементов из списка;
   * AssignImage — описывает правила изменения параметра `image` ресурса.

   Состоит из следующих контейнеров:

   * **manager** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам контроллера.

1. **Gatekeeper-audit** — реализует функционал периодической проверки существующих ресурсов Kubernetes на соответствие политикам безопасности.

   Состоит из следующих контейнеров:

   * **manager** — основной контейнер;
   * **constraint-exporter** — сайдкар-контейнер, предоставляющий дополнительные метрики по кастомным ресурсам `constraints.gatekeeper.sh/*` и `mutations.gatekeeper.sh/*`;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к метрикам `manager` и `constraint-exporter`.

1. **ratify** — состоит из одного контейнера [**ratify**](https://ratify.dev/docs/what-is-ratify) и представляет собой реализацию [Gatekeeper провайдера](https://open-policy-agent.github.io/gatekeeper/website/docs/externaldata) для проверки метаданных используемых артефактов. В DKP этот провайдер применяется для проверки подписи образов контейнеров.

   Компонент ratify доступен в следующих редакциях DKP: SE+, EE, CSE Lite, CSE Pro.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

* **Kube-apiserver**:

  * мониторинг всех ресурсов Kubernetes;
  * работа с кастомными ресурсами ConstraintTemplate, constraints.gatekeeper.sh/*, Assign, AssignImage, AssignMetadata, ModifySet, config.ratify.deislabs.io/\*.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver** — валидация ресурсов Kubernetes и проверка на соответствие заданным правилам безопасности.

1. **Prometheus-main** — сбор метрик модуля.

## Кастомные ресурсы

Модуль `admission-policy-engine` добавляет в платформу DKP кастомные ресурсы, упрощающие настройку наиболее часто встречающихся политик безопасности. Используются следующие [кастомные ресурсы](admission-policy-engine/cr.html):

* OperationPolicy — описывает операционную политику кластера;
* SecurityPolicy — описывает политику безопасности кластера;
* SecurityPolicyException — описывает исключения из политики безопасности кластера.

   Обработкой этих кастомных ресурсов происходит с использованием механизма [hooks](../module-development/structure/#hooks). Подробнее об этом механизме можно ознакомиться в документации [addon-operator](https://flant.github.io/addon-operator/OVERVIEW.html).

   На основе OperationPolicy и SecurityPolicy создаются кастомные ресурсы для [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).
