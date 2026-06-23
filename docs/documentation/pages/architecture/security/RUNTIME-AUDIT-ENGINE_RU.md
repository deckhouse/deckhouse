---
title: Модуль runtime-audit-engine
permalink: ru/architecture/security/runtime-audit-engine.html
lang: ru
search: аудит безопасности, правила аудита, falco, runtime-audit-engine
description: Архитектура модуля runtime-audit-engine в Deckhouse Kubernetes Platform.
---

Модуль [`runtime-audit-engine`](/modules/runtime-audit-engine/) реализует в Deckhouse Kubernetes Platform (DKP) [аудит событий безопасности](./runtime-audit.html), основанный на системе обнаружения угроз [Falco](https://falco.org/). Модуль собирает события ядра Linux и события аудита API Kubernetes (с помощью плагина `k8saudit`), обогащает их метаданными о подах Kubernetes и формирует события безопасности по установленным правилам. Правила аудита определяются с использованием кастомного ресурса [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules).

При включении модуля в неймспейсе `d8-runtime-audit-engine` создаётся ConfigMap `control-plane-configurator` с URL и CA для audit webhook. Модуль [`control-plane-manager`](/modules/control-plane-manager/) обнаруживает этот ConfigMap и настраивает control plane на отправку событий аудита API Kubernetes в модуль `runtime-audit-engine`.

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

- На схеме контейнеры разных подов показаны как взаимодействующие напрямую. Фактически обмен выполняется через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса приводится над стрелкой.
- Поды могут быть запущены в нескольких репликах, однако на схеме каждый под показан в единственном экземпляре.
{% endalert %}

Архитектура модуля [`runtime-audit-engine`](/modules/runtime-audit-engine/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля runtime-audit-engine](../../images/architecture/security/c4-l2-runtime-audit-engine.ru.svg)

## Компоненты модуля

Модуль `runtime-audit-engine` состоит из следующих компонентов:

1. **Runtime-audit-engine** (DaemonSet) — компонент, запускаемый на каждом узле кластера. Он собирает события аудита, проводит проверку правил и экспортирует сработавшие правила их как метрики в формате Prometheus. Данные поступают из ядра Linux через eBPF-драйвер, а также из `containerd` через Unix-сокет.

   Компонент содержит следующие контейнеры:

   - **falco** — основной контейнер, обеспечивающий сбор событий безопасности с узлов кластера и с контейнерных приложений в DKP на основе системы обнаружения угроз [Falco](https://falco.org/). Falco перехватывает системные вызовы (syscalls) из ядра Linux в режиме реального времени, обрабатывает их на основе правил и выводит в stdout результат срабатывания правил аудита;
   - **falcosidekick** — сайдкар-контейнер, получающий события от компонента `falco` и экспортирующий события аудита в формате метрик Prometheus;
   - **rules-loader** — сайдкар-контейнер, выполняющий следующие действия:
      - отслеживает кастомные ресурсы [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) и сохраняет их в общий каталог пода `/etc/falco/rules.d/` для обработки компонентом `falco`;
      - валидирует кастомный ресурс FalcoAuditRules;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC (Role-Based Access Control) для организации защищённого доступа к метрикам компонента.

   {% alert level="warning" %}
   Контейнер `falco` имеет привилегированный доступ к операционной системе каждого узла. Контекст безопасности контейнера включает capabilities `BPF`, `SYS_RESOURCE`, `PERFMON`, `SYS_PTRACE` и `SYS_ADMIN`.
   {% endalert %}

1. **K8s-metacollector** (Deployment) — компонент, выполняющий проксирование запросов к `kube-apiserver` для уменьшения нагрузки на control plane. Также компонент уменьшает объём метаданных, передаваемых в `falco`, оставляя только данные, относящиеся к узлу. K8s-metacollector собирает метаданные из `kube-apiserver` о Pod, Namespace, Deployment, ReplicaSet, ReplicationController и Service.

   Компонент содержит следующие контейнеры:

   - **k8s-metacollector** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам k8s-metacollector.

## Взаимодействия модуля

Модуль `runtime-audit-engine` взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - работа с кастомными ресурсами [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules);
   - мониторинг ресурсов Pod, Namespace, Deployment, ReplicaSet, ReplicationController и Service;
   - авторизация запросов компонентов модуля.

1. **Containerd**:

   - получение метаданных контейнеров;
   - отслеживание событий `containerd`.

1. **Ядро Linux** — перехват системных вызовов (syscalls) из ядра Linux в режиме реального времени.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:

   - отправляет вебхук-запросы на валидацию кастомного ресурса FalcoAuditRules;
   - отправляет события аудита.

1. **Prometheus-main** — собирает метрики модуля.
