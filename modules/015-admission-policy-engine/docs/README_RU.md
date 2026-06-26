---
title: "Модуль admission-policy-engine"
description: Модуль admission-policy-engine Deckhouse позволяет использовать в кластере Kubernetes политики безопасности согласно Kubernetes Pod Security Standards.
---

Модуль `admission-policy-engine` реализует поддержку admission-политик безопасности в кластере Kubernetes.

Admission-политики — это правила, которые применяются к объектам (например Pod и Service) в момент их создания и изменения в кластере (но не в процессе их работы), на основе информации, представленной в их манифесте. Эти политики направлены на формализацию параметров которые разрешены или запрещены в манифестах объектов.

В DKP политики разделены на три категории:

- [Pod Security Standards](#pod-security-standards) — политики, реализующие соответствующие [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).
- [Операционные политики](#операционные-политики) — политики для создания дополнительных требований к объектам, с помощью валидации значений параметров **не связанных напрямую** с безопасностью (например, список допустимых префиксов для образов контейнеров, политика скачивания образов, список необходимых проб для контейнеров и т.д.).
- [Политики безопасности](#политики-безопасности) — политики для создания дополнительных требований к объектам, с помощью валидации значений параметров, связанных с безопасностью (например, доступ контейнеров к IPC- или PID-пространству имен хоста, список привилегий для контейнеров и т.д.).

{% alert level="info" %}
Эти политики дополняют друг друга. Если для одного неймспейса применены несколько политик, выполняется валидация объектов по каждой из них. Если хоть одна политика будет нарушена, объект создан не будет.
{% endalert %}

Помимо политик, которые запрещают использование параметров, не соответствующих заданным требованиям, модуль поддерживает [ресурс SecurityPolicyException](#исключения-из-политик-безопасности), который позволяет создавать точечные исключения из проверок политик безопасности. С помощью этого ресурса можно разрешить использование отдельных параметров для конкретных подов или контейнеров, не изменяя политики безопасности, действующие для всего неймспейса.

## Особенности отображения сообщений о неудачной валидации объектов

В зависимости от способа создания подов есть особенности формирования сообщений от API о неудачной валидации (нарушении установленных политик):

- Если под создается напрямую, ошибка валидации возвращается в ответе от API о неудачной валидации (нарушении политики).
- Если поды создаются через Deployment, создаётся требуемое количество ReplicaSet, которые, в свою очередь, пытаются создать поды. В этом случае ошибка валидации не возвращается в ответе API, а отображается в событиях неймспейса или соответствующего ReplicaSet.

## Валидация подов при изменении политики или добавлении новой

Для всех трех категорий политик (Pod Security Standards, операционные и политики безопасности) не предусмотрено автоматическое пересоздание существующих подов при изменении действующих или добавлении новых политик. Поды, существовавшие до момента внесения изменений в используемую политику или до добавления новой, продолжат работать до перезапуска. А при перезапуске они будут валидироваться по новым правилам.

В модуле `admission-policy-engine` для таких случаев предусмотрены алерты (`kind: ClusterObservabilityAlert`), информирующие о наличии в неймспейсе подов с нарушениями после изменения существующей политики или добавления новой.

Для получения списка алертов используйте команду:

```bash
d8 k get clusterobservabilityalerts
```

Пример ответа:

<!-- markdownlint-disable MD031 -->
```console
NAME                                                  SEVERITY   STATUS   DURATION   SUMMARY                          AGE
SecurityPolicyViolation-f3a77d1dd2175402-1777370195   1          Firing   5h         Alerting PrometheusUnavailable   5h1m
OperationPolicyViolation-9b21d0c871796913-1777370435  1          Firing   6h         Alerting PrometheusUnavailable   6h1m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Для просмотра информации о конкретном алерте используйте команду:

```bash
d8 k get clusterobservabilityalert OperationPolicyViolation-9b21d0c871796913-1777370435 -oyaml
```

{% offtopic title="Пример алерта при нарушении политики Pod Security Standards..." %}

```yaml
kind: ClusterObservabilityAlert
apiVersion: alerts.observability.deckhouse.io/v1alpha1
metadata:
  name: PodSecurityStandardsViolation-91e71759e048a397-1777369535
  resourceVersion: "7454828154578800069"
  creationTimestamp: 2026-04-28T09:45:35Z
  labels:
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
alert:
  labels:
    alertname: PodSecurityStandardsViolation
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
    severity_level: "3"
  annotations:
    description: |-
      You have configured [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/), and one or more running pods are violating these standards.

      To identify violating pods:

      - Run the following Prometheus query:

        ```prometheus
        count by (violating_namespace, violating_name, violation_msg) (
          d8_gatekeeper_exporter_constraint_violations{
            violation_enforcement="deny",
            violating_namespace=~".*",
            violating_kind="Pod",
            source_type="PSS"
          }
        )
        ```

      - Alternatively, check the admission-policy-engine Grafana dashboard.
    plk_markup_format: markdown
    plk_protocol_version: "1"
    summary: At least one pod violates the configured cluster pod security standards.
  expr: (count(d8_gatekeeper_exporter_constraint_violations{source_type="PSS",violating_kind="Pod",violating_namespace=~".*",violation_enforcement="deny"}))
    > 0
  created_by: observability
  rule_group_name: admission-policy-engine-audit-0
status:
  alertStatus: Firing
  silencedBy: []
  startsAt: 2026-04-28T09:45:35Z
  resolvedAt: null
  duration: 20h40m1.015261771s
```

{% endofftopic %}

{% offtopic title="Пример алерта при нарушении операционной политики..." %}

```yaml
kind: ClusterObservabilityAlert
apiVersion: alerts.observability.deckhouse.io/v1alpha1
metadata:
  name: OperationPolicyViolation-9b21d0c871796913-1777370435
  resourceVersion: "7454831929456594373"
  creationTimestamp: 2026-04-28T10:00:35Z
  labels:
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
alert:
  labels:
    alertname: OperationPolicyViolation
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
    severity_level: "3"
  annotations:
    description: >-
      You have configured operation policies for the cluster, and one or more
      existing objects are violating these policies.


      To identify violating objects:


      - Run the following Prometheus query:

        ```prometheus
        count by (violating_namespace, violating_kind, violating_name, violation_msg) (
          d8_gatekeeper_exporter_constraint_violations{
            violation_enforcement="deny",
            source_type="OperationPolicy"
          }
        )
        ```

      - Alternatively, check the admission-policy-engine Grafana dashboard.
    plk_markup_format: markdown
    plk_protocol_version: "1"
    summary: At least one object violates the configured cluster operation policies.
  expr: (count(d8_gatekeeper_exporter_constraint_violations{source_type="OperationPolicy",violation_enforcement="deny"}))
    > 0
  created_by: observability
  rule_group_name: admission-policy-engine-audit-0
status:
  alertStatus: Firing
  silencedBy: []
  startsAt: 2026-04-28T10:00:35Z
  resolvedAt: null
  duration: 20h23m41.023025059s
```

{% endofftopic %}

{% offtopic title="Пример алерта при нарушении политики безопасности..." %}

```yaml
kind: ClusterObservabilityAlert
apiVersion: alerts.observability.deckhouse.io/v1alpha1
metadata:
  name: SecurityPolicyViolation-f3a77d1dd2175402-1777370195
  resourceVersion: "7454830922622307781"
  creationTimestamp: 2026-04-28T09:56:35Z
  labels:
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
alert:
  labels:
    alertname: SecurityPolicyViolation
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
    severity_level: "3"
  annotations:
    description: >-
      You have configured security policies for the cluster, and one or more
      existing objects are violating these policies.


      To identify violating objects:


      - Run the following Prometheus query:

        ```prometheus
        count by (violating_namespace, violating_kind, violating_name, violation_msg) (
          d8_gatekeeper_exporter_constraint_violations{
            violation_enforcement="deny",
            source_type="SecurityPolicy"
          }
        )
        ```

      - Alternatively, check the admission-policy-engine Grafana dashboard.
    plk_markup_format: markdown
    plk_protocol_version: "1"
    summary: At least one object violates the configured cluster security policies.
  expr: (count(d8_gatekeeper_exporter_constraint_violations{source_type="SecurityPolicy",violation_enforcement="deny"}))
    > 0
  created_by: observability
  rule_group_name: admission-policy-engine-audit-0
status:
  alertStatus: Firing
  silencedBy: []
  startsAt: 2026-04-28T09:56:35Z
  resolvedAt: null
  duration: 20h29m21.015479019s
```

{% endofftopic %}

## Pod Security Standards

[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) (`PSS`) — это официальный стандарт Kubernetes, который определяет три уровня безопасности для подов, ограничивая их привилегии. Ограничение происходит с помощью запрета установки определенных параметров в манифесте пода.

Используется многослойная структура — каждый более высокий уровень защиты использует все правила предыдущего уровня и добавляет свои.

В `PSS` регламентированы следующие уровни защиты (политики):

- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений (отсутствие ограничений).
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода.
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жёсткие требования к подам.

{% alert level="info" %}
В Deckhouse Kubernetes Platform эти политики реализуются средствами Gatekeeper и контролируются admission-контроллерами модуля `admission-policy-engine`, а не контролером [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) от Kubernetes. Из Kubernetes взяты только описания политик.
{% endalert %}

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

Политика PSS для неймспейса включается через добавление на него специального лейбла `security.deckhouse.io/pod-policy=<POLICY_NAME>`.
Политику по умолчанию можно переопределить глобально ([в настройках модуля](configuration.html#parameters-podsecuritystandards-defaultpolicy)).

{% alert level="info" %}
Модуль не применяет политики к системным неймспейсам.
{% endalert %}

{% alert level="info" %}
При включении [модуля `multitenancy-manager`](/modules/multitenancy-manager/) он создаёт свои объекты OperationPolicy (например, в неймспейсе `default`). На них не влияют [настройки `podSecurityStandards`](configuration.html#parameters-podsecuritystandards).
{% endalert %}

Пример установки политики `Restricted` для всех подов в неймспейсе `my-namespace`:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

Дополнительно возможна настройка режима работы политики.
Поддерживаются следующие режимы:

- `deny` — запретить запуск подов, не удовлетворяющих политике;
- `warn` — запускать поды, не удовлетворяющие политике, но выдавать предупреждение.
- `dryrun` — запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

Настройка режима работы политики производится путем установки лейбла `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` на соответствующем неймспейсе.
Чтобы задать режим работы политик глобально, используйте параметр [`enforcementaction`](configuration.html#parameters-podsecuritystandards-enforcementaction).

Пример установки "warn" режима политик PSS для всех подов в неймспейсе `my-namespace`:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

## Операционные политики

Операционные политики — это правила, направленные на достижение лучших практик безопасности приложений, но **не относящиеся** напрямую к валидации классических параметров, связанных с безопасностью (например, список допустимых префиксов для образов контейнеров, политика скачивания образов, список необходимых проб для контейнеров и т.д.).

Операционные политики описываются с помощью кастомного ресурса [`OperationPolicy`](/modules/admission-policy-engine/cr.html#operationpolicy).
В нём каждый параметр отвечает за отдельную проверку, применяемую к ресурсам.
Использование кастомного ресурса OperationPolicy позволяет создавать дополнительные требования к создаваемым ресурсам (высокоуровневые декларативные операционные политики) без явной работы с Gatekeeper.

Рекомендуется устанавливать следующий минимальный набор операционных политик:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  enforcementAction: Deny
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.ru
    requiredResources:
      limits:
        - memory
      requests:
        - cpu
        - memory
    disallowedImageTags:
      - latest
    requiredProbes:
      - livenessProbe
      - readinessProbe
    maxRevisionHistoryLimit: 3
    imagePullPolicy: Always
    priorityClassNames:
    - production-high
    - production-low
    checkHostNetworkDNSPolicy: true
    checkContainerDuplicates: true
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          custom-operation-policy/enabled: "true"
```

Применение политики реализовано через настройки, расположенные в параметре `spec.match`.

При указании:

```yaml
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          custom-operation-policy/enabled: "true"
```

Для применения приведённой политики достаточно добавить лейбл `custom-operation-policy/enabled: "true"` на желаемый неймспейс.  
В отличие от `PSS`, название лейбла может быть любым. Требуется лишь совпадение лейбла в селекторе политик и соответствующего неймспейса.

Более подробную информацию об использовании селекторов вы можете прочитать в [описании настройки селекторов](/modules/admission-policy-engine/docs/faq.html#как-настроить-селекторы-политик).

Для политики также возможно указание применяемого действия.
Для этого используется параметр `spec.enforcementAction`.
Поддерживаются следующие режимы:

- `Deny` — запретить запуск подов не удовлетворяющих политике;
- `Warn` — запускать поды не удовлетворяющие политике, но выдавать предупреждение.
- `Dryrun` — запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

Основываясь на этом примере, вы можете создать собственную политику с необходимыми настройками.

## Политики безопасности

Политики безопасности — это правила, направленные на достижение лучших практик безопасности приложений с помощью валидации значений параметров, связанных с безопасностью (например, доступ контейнеров к IPC- или PID-пространству имен хоста, список привилегий для контейнеров и т.д.).

Политики безопасности описываются с помощью кастомного ресурса [`SecurityPolicy`](/modules/admission-policy-engine/cr.html#securitypolicy).
В нём каждый параметр отвечает за отдельную проверку применяемую к ресурсам.
С помощью этого ресурса возможно сконструировать политику безопасности аналогичную политике PSS любого уровня.
Использование кастомного ресурса SecurityPolicy позволяет создавать дополнительные требования к создаваемым ресурсам (высокоуровневые декларативные политики безопасности) без явной работы с Gatekeeper.

Пример политики безопасности:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: mypolicy
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: true
    allowHostNetwork: true
    allowHostPID: false
    allowPrivileged: false
    allowPrivilegeEscalation: false
    allowedFlexVolumes:
    - driver: vmware
    allowedHostPorts:
    - max: 4000
      min: 2000
    allowedProcMount: Unmasked
    allowedAppArmor:
    - unconfined
    allowedUnsafeSysctls:
    - kernel.*
    allowedVolumes:
    - hostPath
    - projected
    fsGroup:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - ALL
    runAsGroup:
      ranges:
      - max: 500
        min: 300
      rule: RunAsAny
    runAsUser:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    seccompProfiles:
      allowedLocalhostFiles:
      - my_profile.json
      allowedProfiles:
      - Localhost
    supplementalGroups:
      ranges:
      - max: 133
        min: 129
      rule: MustRunAs
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy: mypolicy
```

{% alert level="warning" %}
Параметры `allowPrivilegeEscalation` и `allowPrivileged` по умолчанию имеют значение `false` — даже если не указаны явно. Это означает, что контейнеры не смогут запускаться в привилегированном режиме или повышать привилегии. Чтобы разрешить такое поведение, задайте параметр в `true`.
{% endalert %}

Применение политики реализовано через настройки расположенные в параметре `spec.match`.

При указании:

```yaml
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy: mypolicy
```

Для применения приведённой политики достаточно добавить лейбл `security-policy: mypolicy` на желаемый неймспейс.  
В отличие от `PSS`, название лейбла может быть любым. Требуется лишь совпадение лейбла в селекторе политик и соответствующего неймспейса.

Более подробную информацию о использовании селекторов вы можете прочитать в [описании настройки селекторов](/modules/admission-policy-engine/docs/faq.html#как-настроить-селекторы-политик).

Для политики также возможно указание применяемого действия.
Для этого используется параметр `spec.enforcementAction`.
Поддерживаются следующие режимы:

- `Deny` — запретить запуск подов, не удовлетворяющих политике;
- `Warn` — запускать поды, не удовлетворяющие политике, но выдавать предупреждение.
- `Dryrun` — запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

## Исключения из политик безопасности

[SecurityPolicyException](cr.html#securitypolicyexception) — это ресурс, позволяющий создавать точечные исключения из проверок политик безопасности для отдельных подов и контейнеров. Он позволяет не отключать проверки для всего неймспейса, а описывать только необходимые исключения из конкретного правила для пода или контейнера.

### Добавление исключений

Чтобы добавить исключения для пода или контейнера, выполните следующее:

1. Создайте объект [SecurityPolicyException](cr.html#securitypolicyexception), описав необходимые исключения.

   Рекомендуется документировать причину каждого исключения в поле `metadata` соответствующего правила (например, `metadata.description`). Это упрощает последующие аудит и сопровождение.

2. В шаблоне пода (обычно через поле `spec.template.metadata.labels` ресурса Deployment, StatefulSet или DaemonSet) укажите один следующих лейблов со ссылкой на исключение:
   - `security.deckhouse.io/security-policy-exception: <exception-name>` — исключение для всего пода;
   - `security.deckhouse.io/security-policy-exception.container.<container-name>: <exception-name>` — исключение для конкретного контейнера.

Приоритет выбора исключения для контейнера:

1. Сначала проверяется лейбл `security.deckhouse.io/security-policy-exception.container.<container-name>`.
1. Если лейбл для конкретного контейнера отсутствует, используется исключение из `security.deckhouse.io/security-policy-exception`.

{% alert level="warning" %}
Если для контейнера задан отдельный лейбл, но он указывает на несуществующий или некорректный объект SecurityPolicyException, он всё равно имеет приоритет над общим лейблом и может привести к запрету размещения пода.
{% endalert %}

### Пример конфигурации

Для примера рассмотрим под, которому требуется:

- разрешение на использование настройки [`hostNetwork`](/products/kubernetes-platform/documentation/v1/user/security/pod-settings.html#hostnetwork) всему поду;
- разрешение на использование настройки [`privileged`](/products/kubernetes-platform/documentation/v1/user/security/pod-settings.html#privileged) только для контейнера `sample-init`.

Без использования ресурса SecurityPolicyException для разрешения этих параметров потребовалось бы создать пользовательскую политику безопасности, допускающую их использование для всех подов в кластере.

При использовании SecurityPolicyException достаточно создать следующие ресурсы:

- Исключение для разрешения параметра `hostNetwork`:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: SecurityPolicyException
  metadata:
    name: allow-hostnetwork-pod
  spec:
    network:
      hostNetwork:
        allowedValue: true
        metadata:
          description: >-
            Pod requires host network mode for node-level network diagnostics.
  ```

- Исключение для разрешения параметра `privileged` в контейнере `sample-init`:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: SecurityPolicyException
  metadata:
    name: allow-privileged-init-container
  spec:
    securityContext:
      privileged:
        allowedValue: true
        metadata:
          description: >-
            Container init requires privileged mode to access host-level networking features.
  ```

После этого необходимо добавить соответствующие лейблы в шаблоне пода:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
spec:
  template:
    metadata:
      labels:
        # Общее исключение, применяемое ко всему поду.
        security.deckhouse.io/security-policy-exception: allow-hostnetwork-pod
        # Исключение для контейнера sample-init.
        security.deckhouse.io/security-policy-exception.container.sample-init: allow-privileged-init-container
    spec:
      hostNetwork: true         
    ...
    containers:
      - name: sample-init 
        securityContext:
          privileged: true 
```

## Изменение ресурсов Kubernetes

Модуль позволяет использовать [кастомные ресурсы Gatekeeper](gatekeeper-cr.html) для модификации объектов в кластере, такие как:

- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — для изменения секции `metadata` в ресурсе;
- [Assign](gatekeeper-cr.html#assign) — для изменения других полей, кроме `metadata`;
- [ModifySet](gatekeeper-cr.html#modifyset) — для добавления или удаления значений из списка, например аргументов для запуска контейнера.
- [AssignImage](gatekeeper-cr.html#assignimage) — для изменения параметра `image` ресурса.

Подробнее про доступные варианты можно прочитать в документации [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).
