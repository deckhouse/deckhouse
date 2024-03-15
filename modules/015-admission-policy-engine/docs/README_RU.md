---
title: "Модуль admission-policy-engine"
description: Модуль admission-policy-engine Deckhouse позволяет использовать в кластере Kubernetes политики безопасности согласно Kubernetes Pod Security Standards.
---

Позволяет использовать в кластере политики безопасности согласно [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) Kubernetes. Модуль для работы использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

Pod Security Standards определяют три политики, охватывающие весь спектр безопасности. Эти политики являются кумулятивными, то есть состоящими из набора политик, и варьируются по уровню ограничений от «неограничивающего» до «ограничивающего значительно».

{% alert level="info" %}
Модуль не применяет политики к системным пространствам имен.
{% endalert %}

Список политик, доступных для использования:
- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений;
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жесткие требования к подам.

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

Политика кластера используемая по умолчанию определяется следующим образом:
- При установке Deckhouse версии **ниже v1.55**, для всех несистемных пространств имен используется политика по умолчанию `Privileged`;
- При установке Deckhouse версии **v1.55 и выше**, для всех несистемных пространств имен используется политика по умолчанию `Baseline`;

**Обратите внимание,** что обновление Deckhouse в кластере на версию v1.55 не вызывает автоматической смены политики по умолчанию.

Политику по умолчанию можно переопределить как глобально ([в настройках модуля](configuration.html#parameters-podsecuritystandards-defaultpolicy)), так и для каждого пространства имен отдельно (лейбл `security.deckhouse.io/pod-policy=<POLICY_NAME>` на соответствующем пространстве имен).

Пример установки политики `Restricted` для всех подов в пространстве имен `my-namespace`:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

По умолчанию, политики Pod Security Standards применяются в режиме "Deny" и поды приложений, не удовлетворяющие данным политикам, не смогут быть запущены. Режим работы политик может быть задан как глобально для кластера так и для каждого namespace отдельно. Что бы задать режим работы политик глобально используйте [configuration](configuration.html#parameters-podsecuritystandards-enforcementaction). В случае если необходимо переопределить глобальный режим политик для определенного namespace, допускается использовать лейбл `security.deckhouse.io/pod-policy-action =<POLICY_ACTION>` на соответствующем namespace. Список допустимых режимом политик состоит из: "dryrun", "warn", "deny".

Пример установки "warn" режима политик PSS для всех подов в пространстве имен `my-namespace`:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

Предлагаемые модулем политики могут быть расширены. Примеры расширения политик можно найти в [FAQ](faq.html).

### Операционные политики

Модуль предоставляет набор операционных политик и лучших практик для безопасной работы ваших приложений.
Мы рекомендуем устанавливать следующий минимальный набор операционных политик:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.io
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
          operation-policy.deckhouse.io/enabled: "true"
```

Для применения приведенной политики достаточно навесить лейбл `operation-policy.deckhouse.io/enabled: "true"` на желаемый namespace. Политика, приведенная в примере, рекомендована для использования командой Deckhouse. Аналогичным образом вы можете создать собственную политику с необходимыми настройками.

### Политики безопасности

Модуль предоставляет возможность определять политики безопасности применимо к приложениям (контейнерам), запущенным в кластере.

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
          enforce: mypolicy
```

Для применения приведенной политики достаточно навесить лейбл `enforce: "mypolicy"` на желаемый namespace.

### Изменение ресурсов Kubernetes

Модуль также позволяет использовать custom resource'ы Gatekeeper для легкой модификации объектов в кластере, такие как:
- `AssignMetadata` — для изменения секции metadata в ресурсе;
- `Assign` — для изменения других полей, кроме metadata;
- `ModifySet` — для добавления или удаления значений из списка, например аргументов для запуска контейнера.

Пример:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: demo-annotation-owner
spec:
  match:
    scope: Namespaced
    namespaces: ["default"]
    kinds:
    - apiGroups: [""]
      kinds: ["Pod"]
  location: "metadata.annotations.foo"
  parameters:
    assign:
      value:  "bar"
```

Подробнее про доступные варианты можно прочитать в документации [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).
