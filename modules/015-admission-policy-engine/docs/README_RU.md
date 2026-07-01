---
title: "Модуль admission-policy-engine"
description: Модуль admission-policy-engine Deckhouse позволяет использовать в кластере Kubernetes политики безопасности согласно Kubernetes Pod Security Standards.
---

Модуль `admission-policy-engine` реализует поддержку admission-политик безопасности в кластере Kubernetes.

Admission-политики - это правила, которые применяются к объектам (например `Pod` и `Service`) в момент их создания и изменения в кластере (но не в процессе их работы), на основе информации представленной в их манифесте. Данные политики направлены на формализацию параметров которые разрешены или запрещены в манифестах объектов.

Политики разделены на три категории:
- `Pod Security Standards`;
- Политики безопасности;
- Операционные политики;

## Pod Security Standards

[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) (`PSS`) — это официальный стандарт Kubernetes, который определяет три уровня безопасности для подов, ограничивая их привилегии. Ограничение происходит с помощью запрета установки определенных параметров в манифесте пода.

Используется многослойная структура - каждый более высокий уровень защиты использует все правила предыдущего уровня и добавляет свои.

Регламентированы следующие уровни защиты:
- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений (отсутствие ограничений);
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жёсткие требования к подам.

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

Настройка политик PSS для пространств имен осуществляется через установку специального лейбла `security.deckhouse.io/pod-policy=<POLICY_NAME>` на соответствующем пространстве имен.
Политику по умолчанию можно переопределить глобально ([в настройках модуля](configuration.html#parameters-podsecuritystandards-defaultpolicy)).

{% alert level="info" %}
Модуль не применяет политики к системным пространствам имен.
{% endalert %}

{% alert level="info" %}
При включении [модуль `multitenancy-manager`](/modules/multitenancy-manager/) создаёт свои объекты OperationPolicy (например, в неймспейсе `default`). На них не влияют [настройки `podSecurityStandards`](configuration.html#parameters-podsecuritystandards).
{% endalert %}

Пример установки политики `Restricted` для всех подов в пространстве имен `my-namespace`:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

Дополнительно возможна настройка режима работы политики.
Поддерживаются следующие режимы:
- `deny` - запретить запуск подов, не удовлетворяющих политике;
- `warn` - запускать поды, не удовлетворяющие политике, но выдавать предупреждение.
- `dryrun` - запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

Настройка режима работы политики производится путем установки лейбла `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` на соответствующем пространстве имен.
Чтобы задать режим работы политик глобально, используйте параметр [`enforcementaction`](configuration.html#parameters-podsecuritystandards-enforcementaction).

Пример установки "warn" режима политик PSS для всех подов в пространстве имен `my-namespace`:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

## Операционные политики

Операционные политики - это правила направленные на достижение лучших практик безопасности приложений, но не относящихся напрямую к валидации классических параметров связанных с безопасностью.

Операционные политики описываются с помощью кастомного ресурса [`OperationPolicy`](/modules/admission-policy-engine/cr.html#operationpolicy).
В данном ресурсе каждый параметр отвечает за отдельную проверку применяемую к ресурсам.

Мы рекомендуем устанавливать следующий минимальный набор операционных политик:

```yaml
---
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

Применение политики реализовано через настройки расположенные в параметре `spec.match`.

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

Более подробную информацию об использовании селекторов вы можете прочитать в [описании настройки селекторов](/modules/admission-policy-engine/faq.html#как-настроить-селекторы-политик).

Для политики также возможно указание применяемого действия.
Для этого используется параметр `spec.enforcementAction`.
Поддерживаются следующие режимы:
- `Deny` - запретить запуск подов не удовлетворяющих политике;
- `Warn` - запускать поды не удовлетворяющие политике, но выдавать предупреждение.
- `Dryrun` - запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

Основываясь на этом примере, вы можете создать собственную политику с необходимыми настройками.

## Политики безопасности

Политики безопасности - это правила направленные на достижение лучших практик безопасности приложений с помощью валидации значений параметров связанных с безопасностью.

Политики безопасности описываются с помощью кастомного ресурса [`SecurityPolicy`](/modules/admission-policy-engine/cr.html#securitypolicy).
В данном ресурсе каждый параметр отвечает за отдельную проверку применяемую к ресурсам.
Используя данный ресурс возможно сконструировать политику безопасности аналогичную политике PSS любого уровня.

Пример политики безопасности:

```yaml
---
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
В отличии от `PSS`, название лейбла может быть любым. Требуется лишь совпадение лейбла в селекторе политик и соответствующего неймспейса.

Более подробную информацию о использовании селекторов вы можете прочитать в [описании настройки селекторов](/modules/admission-policy-engine/faq.html#как-настроить-селекторы-политик).

Для политики также возможно указание применяемого действия.
Для этого используется параметр `spec.enforcementAction`.
Поддерживаются следующие режимы:
- `Deny` - запретить запуск подов, не удовлетворяющих политике;
- `Warn` - запускать поды, не удовлетворяющие политике, но выдавать предупреждение.
- `Dryrun` - запускать поды не удовлетворяющие политике, не выдавать предупреждение пользователю, но фиксировать нарушения в отчетах безопасности;

## Изменение ресурсов Kubernetes

Модуль позволяет использовать [кастомные ресурсы Gatekeeper](gatekeeper-cr.html) для модификации объектов в кластере, такие как:
- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — для изменения секции `metadata` в ресурсе;
- [Assign](gatekeeper-cr.html#assign) — для изменения других полей, кроме `metadata`;
- [ModifySet](gatekeeper-cr.html#modifyset) — для добавления или удаления значений из списка, например аргументов для запуска контейнера.
- [AssignImage](gatekeeper-cr.html#assignimage) — для изменения параметра `image` ресурса.

Подробнее про доступные варианты можно прочитать в документации [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).
