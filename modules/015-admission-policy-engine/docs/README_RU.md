---
title: "Модуль admission-policy-engine"
description: Модуль admission-policy-engine Deckhouse позволяет использовать в кластере Kubernetes политики безопасности согласно Kubernetes Pod Security Standards.
---

Позволяет использовать в кластере политики безопасности согласно [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) Kubernetes. Модуль для работы использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

Pod Security Standards определяют три политики, охватывающие весь спектр безопасности. Эти политики являются кумулятивными, то есть состоящими из набора политик, и варьируются по уровню ограничений от «неограничивающего» до «ограничивающего значительно».

Список политик, предлагаемых модулем для использования:
- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений (используется по умолчанию);
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жесткие требования к подам.

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/).

Для применения политики достаточно установить лейбл `security.deckhouse.io/pod-policy=<POLICY_NAME>` на соответствующее пространство имен.

Пример установки политики `Restricted` для всех подов в пространстве имен `my-namespace`:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
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
