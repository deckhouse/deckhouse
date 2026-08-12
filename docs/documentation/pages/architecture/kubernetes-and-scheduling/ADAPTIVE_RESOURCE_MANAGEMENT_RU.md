---
title: Модуль adaptive-resource-management
permalink: ru/architecture/kubernetes-and-scheduling/adaptive-resource-management.html
lang: ru
search: adaptive-resource-management
description: Архитектура модуля adaptive-resource-management в Deckhouse Kubernetes Platform.
---

Модуль [`adaptive-resource-management`](/modules/adaptive-resource-management/) позволяет автоматизировать подбор resource requests и limits для рабочих нагрузок на основе рекомендаций Vertical Pod Autoscaler (VPA).

Для этого модуль разворачивает контроллер **AutoVPA**, основанный на проекте [Goldilocks](https://github.com/FairwindsOps/goldilocks), с доработками Deckhouse Kubernetes Platform (DKP). Контроллер автоматически создает и поддерживает объекты VPA для рабочих нагрузок в выбранных неймспейсах и предоставляет рекомендации по настройке ресурсов.

Основные возможности:

* Автоматическое создание объектов VPA для Deployment, StatefulSet, DaemonSet и Job в управляемых неймспейсах.
* Гибкий выбор неймспейсов: управление всеми неймспейсами, неймспейсами с определённым лейблом или соответствующими определённому label-селектору.
* Комбинированный режим выбора неймспейсов по label-селектору и специальному лейблу `autovpa.deckhouse.io/enabled`.
* Автоматическое создание объектов VPA в режиме рекомендаций без изменения манифестов приложений.
* Настройка VPA на уровне отдельных нагрузок и неймспейсов через аннотации и лейблы `autovpa.deckhouse.io/*`.
* Минимальное потребление ресурсов: контроллер работает в одной реплике и предъявляет низкие требования к CPU и памяти.

Подробнее с настройками модуля и примерами его использования можно ознакомиться [в разделе документации модуля](/modules/adaptive-resource-management/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`adaptive-resource-management`](/modules/adaptive-resource-management/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP показаны на следующей диаграмме:

![Архитектура модуля adaptive-resource-management](../../images/architecture/kubernetes-and-scheduling/c4-l2-adaptive-resource-management.ru.png)

## Компоненты модуля

Модуль `adaptive-resource-management` состоит из одного компонента **autovpa-controller**, включающего основной контейнер **autovpa**.

## Взаимодействия модуля

Модуль взаимодействует с компонентом **kube-apiserver** для создания объектов VPA под рабочие нагрузки.
