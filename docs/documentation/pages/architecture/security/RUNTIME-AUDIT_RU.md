---
title: Архитектура рантайм-аудита
permalink: ru/architecture/security/runtime-audit.html
lang: ru
search: рантайм-аудит, аудит рантайма, правила аудита, falco
description: Архитектура рантайм-аудита в Deckhouse Kubernetes Platform.
---

Рантайм-аудит Deckhouse Kubernetes Platform (DKP) основан на системе обнаружения угроз [Falco](https://falco.org/).
Этот механизм анализирует события ядра Linux и аудит событий Kubernetes API для выявления подозрительной активности
в работающих контейнерах и кластере в целом.

DKP запускает объединённые в DaemonSet агенты Falco на каждом узле,
после чего те приступают к сбору системных вызовов ОС и данных, полученных в ходе аудита событий Kubernetes.

{% alert level="info" %}
Разработчики Falco рекомендуют запускать его как systemd-сервис,
что может быть затруднительно в кластерах Kubernetes с поддержкой автомасштабирования.
В DKP реализованы дополнительные механизмы безопасности, такие как мультитенантность и политики контроля ресурсов,
которые в сочетании с использованием DaemonSet обеспечивают высокий уровень защиты.
{% endalert %}

![Агенты Falco на узлах кластера DKP](../../images/runtime-audit-engine/falco_daemonset.svg)
<!--- Source: https://docs.google.com/drawings/d/1NZ91z8NXNiuS50ybcMoMsZI3SbQASZXJGLANdaNNm_U --->

На каждом узле кластера запускается под Falco со следующими компонентами:

- `falco` — собирает события, обогащает их метаданными и отправляет в stdout;
- `rules-loader` — собирает данные с правилами из [кастомных ресурсов FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules)
  и сохраняет их в общую директорию;
- [`falcosidekick`](https://github.com/falcosecurity/falcosidekick) — принимает события от `falco`
  и экспортирует их в виде метрик во внешние системы;
- `kube-rbac-proxy` — защищает эндпоинт метрик `falcosidekick` от неавторизованного доступа.

Подробнее с архитектурой модуля [`runtime-audit-engine`](/modules/runtime-audit-engine/), реализующего аудит событий безопасности DKP, можно ознакомиться [в разделе архитектуры модуля](./runtime-audit-engine.html).

## Правила аудита

Для анализа событий безопасности применяются правила, определяющие критерии подозрительного поведения.
Каждое правило представляет собой выражение, содержащее определённое условие
и написанное в соответствии [с синтаксисом условий Falco](https://falco.org/docs/concepts/rules/conditions/).

### Встроенные правила

В DKP предусмотрены следующие виды встроенных правил:

- **правила для аудита Kubernetes**, которые помогают выявить проблемы с безопасностью DKP и самим механизмом аудита.
  Эти правила расположены в контейнере `falco` по пути `/etc/falco/k8s_audit_rules.yaml`;
- **нормативные правила**, удовлетворяющие требованиям приказа ФСТЭК России №118 от 4 июля 2022 г.
  «Требования по безопасности информации к средствам контейнеризации».
  Эти правила `fstec` описаны в формате [кастомного ресурса FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules).

### Пользовательские правила

Для добавления пользовательских правил используется [кастомный ресурс FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules).

У каждого агента Falco есть сайдкар-контейнер с экземпляром сервиса [`shell-operator`](https://github.com/flant/shell-operator).
Этот экземпляр считывает правила из ресурсов Kubernetes, конвертирует их в правила Falco
и сохраняет правила в директорию `/etc/falco/rules.d/` в поде.
При добавлении нового правила Falco автоматически обновляет конфигурацию.

![Работа shell-operator с правилами Falco](../../images/runtime-audit-engine/falco_shop.svg)
<!--- Source: https://docs.google.com/drawings/d/13MFYtiwH4Y66SfEPZIcS7S2wAY6vnKcoaztxsmX1hug --->
