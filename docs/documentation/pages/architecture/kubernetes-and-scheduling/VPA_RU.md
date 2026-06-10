---
title: "Вертикальное масштабирование"
permalink: ru/architecture/kubernetes-and-scheduling/vpa.html
lang: ru
search: архитектура автомасштабирования, вертикальное масштабирование, оптимизация ресурсов, масштабирование подов, vpa, vertical pod autoscaler, vertical-pod-autoscaler
description: Режимы работы и ограничения VPA в Deckhouse Kubernetes Platform.
relatedLinks:
  - title: "Включение вертикального масштабирования"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/app-scaling/vpa.html
---

Модуль [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) обеспечивает работу [Vertical Pod Autoscaler (VPA)](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler) в Deckhouse Kubernetes Platform (DKP).

Подробнее о настройках модуля и примерах его использования можно узнать [в соответствующем разделе документации](/modules/vertical-pod-autoscaler/configuration.html).

## Режимы работы VPA

VPA может работать в двух режимах:

- Автоматическое изменение запросов ресурсов:

  - **InPlaceOrRecreate** (по умолчанию, режим GA, работает в Kubernetes 1.33 и новее) — VPA пытается изменить ресурсы без пересоздания подов. Если обновить ресурсы «на месте» (in-place) невозможно, VPA переходит к схеме, аналогичной режиму **Recreate**: под, для которого невозможно обновить ресурсы, вытесняется, и вместо него контроллер создает новый под с обновленными ресурсами.

    > Чтобы использовать режим **InPlaceOrRecreate** в Kubernetes до версии 1.33, включите функцию (feature gate) `InPlacePodVerticalScaling` в [настройках модуля `control-plane-manager`](/modules/control-plane-manager/configuration.html#parameters-enabledfeaturegates).

  - **Auto** — устаревший режим. Его поддержка будет прекращена в будущих версиях VPA API. Рекомендуется перейти на явный режим работы, например **InPlaceOrRecreate**, **Recreate** или **Initial**.

  - **Recreate** — VPA может изменять ресурсы у работающих подов, перезапуская их. В случае одного пода (`replicas: 1`) это приведет к недоступности сервиса на время перезапуска. VPA не пересоздает поды, если они были созданы без контроллера.

- Только рекомендации, без изменения ресурсов:

  - **Initial** — ресурсы подов изменяются только при их создании, но не в процессе работы.

  - **Off** — VPA не меняет ресурсы автоматически. Однако, с его помощью можно просматривать рекомендуемые ресурсы с помощью команды `d8 k describe vpa`.

При использовании VPA и включении соответствующего режима, запрашиваемые ресурсы устанавливаются автоматически на основе данных из Prometheus. Также возможно настроить систему таким образом, чтобы она только рекомендовала ресурсы, но не изменяла их. Подробнее про включение и настройку VPA можно почитать в [разделе «Администрирование»](../../admin/configuration/app-scaling/vpa.html).

## Ограничения VPA

Перед использованием вертикального масштабирования (VPA) необходимо учитывать ряд ограничений:

- Перезапуск подов при изменении ресурсов:
  - При использовании режимов, допускающих пересоздание подов, VPA может пересоздать под, если обновить запрашиваемые ресурсы без перезапуска невозможно;
  - Новый под может быть назначен на другой узел.

- Совместимость с [Horizontal Pod Autoscaler (HPA)](../../admin/configuration/app-scaling/hpa.html):
  - VPA не рекомендуется использовать совместно с HPA, выполняющим масштабирование по CPU или памяти;
  - VPA можно использовать совместно с HPA, выполняющим масштабирование по custom- или external-метрикам.

- Проблемы с большими кластерами — VPA может работать и в больших кластерах, но нагрузка на VPA возрастает при росте числа подов.

- Проблемы с Pending-подами — VPA может рекомендовать ресурсы выше доступных в кластере, из-за чего поды могут застрять в статусе `Pending`.

- Проблемы при удалении VPA — если удалить VPA или отключить его (режим `Off`), ресурсы останутся в последнем измененном значении. Это может привести к путанице, когда в Helm указаны одни ресурсы, в контроллере — другие, а у подов — третьи.

- Использование нескольких VPA-ресурсов на один под — может привести к непредсказуемому поведению.

{% alert level="warning" %}
При использовании VPA рекомендуется настроить [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/).
{% endalert %}

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP показаны на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля vertical-pod-autoscaler](../../../images/architecture/kubernetes-and-scheduling/c4-l2-vertical-pod-autoscaler.ru.png)

## Компоненты модуля

Модуль `vertical-pod-autoscaler` состоит из следующих компонентов:

1. **Vpa-admission-controller** (Deployment) — контроллер [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), обслуживающий работу с кастомным ресурсом [VerticalPodAutoscaler](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler).

   Компонент vpa-admission-controller выполняет следующие действия:

   - валидирует кастомные ресурсы VerticalPodAutoscaler;
   - при создании пода (если для VPA не установлен режим [**Off**](./vpa.html#режимы-работы-vpa)) контроллер автоматически задаёт или меняет значения `requests` и `limits` в контейнерах, оптимизируя их по текущим рекомендациям. Значения `limits` контроллер изменяет только в том случае, если в параметре [`spec.resourcePolicy.containerPolicies.controlledValues`](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies-controlledvalues) политики управления ресурсами установлено значение `RequestsAndLimits`.

   Состоит из следующих контейнеров:

   * **admission-controller** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для защищенного доступа к метрикам admission-controller. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

1. **Vpa-updater** (Deployment) — компонент [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), проверяющий, что у подов с VPA выставлены корректные ресурсы. Vpa-updater выполняет in-place-обновление ресурсов через субресурс Kubernetes `pods/resize`, а если это невозможно или не подходит по политике управления ресурсами, вытесняет под.

   Состоит из следующих контейнеров:

   * **updater** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для защищенного доступа к метрикам updater. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

1. **Vpa-recommender** (Deployment) — компонент [VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler), определяющий рекомендации для `requests` на основе информации о прошлом и текущем потреблении ресурсов подами.

    Vpa-admission-controller и vpa-updater пересчитывают значения `limits` пропорционально `requests` в том случае, если в параметре [`spec.resourcePolicy.containerPolicies.controlledValues`](/modules/vertical-pod-autoscaler/cr.html#verticalpodautoscaler-v1-spec-resourcepolicy-containerpolicies-controlledvalues) политики управления ресурсами установлено значение `RequestsAndLimits`.

   Состоит из следующих контейнеров:

   * **recommender** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для защищенного доступа к метрикам recommender. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   * наблюдение за стандартными ресурсами ConfigMap, Node, LimitRange, Pod, а также за кастомными ресурсами VerticalPodAutoscaler и VerticalPodAutoscalerCheckpoint;
   * получение текущего потребления ресурсов через [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md);
   * вытеснение работающих подов при несоответствии спецификации ресурсов и рекомендуемых значений;
   * авторизация запросов на получение метрик.

1. **Prometheus** — получение истории метрик потребления ресурсов подом.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:
     - валидация кастомных ресурсов VerticalPodAutoscaler;
     - изменение `requests` и `limits` в спецификации подов.
1. **Prometheus** — собирает метрики модуля.
