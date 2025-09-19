---
title: Политики безопасности
permalink: ru/virtualization-platform/documentation/admin/platform-management/security/policies.html
lang: ru
---

Deckhouse Virtualization Platform (DVP) позволяет управлять безопасностью приложений в кластере с помощью набора политик,
соответствующих модели [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) и дополнительно расширяемых через встроенные механизмы DVP.

Для реализации политик безопасности в DVP используется [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

## Применение Pod Security Standards

В DVP поддерживаются три уровня политики безопасности:

- `privileged` — неограничивающая политика с максимально широким уровнем разрешений;
- `baseline` — минимально ограничивающая политика,
  которая предотвращает наиболее известные и популярные способы повышения привилегий.
  Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `restricted` — политика со значительными ограничениями. Предъявляет самые жесткие требования к подам.

### Политика по умолчанию

Используемая по умолчанию политика определяется следующим образом:

- в версиях DVP до v1.55 политика по умолчанию — `privileged`;
- начиная с версии DVP v1.55, политика по умолчанию — `baseline`.

{% alert level="info" %}
При обновлении DVP на версию v1.55 или выше политика по умолчанию не изменится автоматически.
{% endalert %}

### Назначение политики

Варианты назначения политики:

- глобально — с помощью [параметра `settings.podSecurityStandards.defaultPolicy`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) модуля `admission-policy-engine`;
- для конкретного пространства имён — с помощью лейбла `security.deckhouse.io/pod-policy=<POLICY_NAME>`.

  Пример команды для назначения политики `restricted` на все поды в пространстве имён `my-namespace`:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
  ```

### Режимы применения политики

Допустимые режимы применения политик:

- `deny` — запрещает выполнений действий.
- `dryrun` — не влияет на выполнение действий и используется для отладки.
  Информацию о событиях можно посмотреть в Grafana или в консоли с помощью команды `kubectl`.
- `warn` — работает как `dryrun`, но дополнительно выводит предупреждение с указанием причины,
  по которой бы произошёл запрет действия в режиме `deny`.

По умолчанию, политики Pod Security Standards в DVP применяются в режиме `deny`.
В этом режиме поды приложений, не удовлетворяющие политикам, не могут быть запущены в кластере.

Как и в случае с назначением политик, режим их применения можно задать:

- глобально — с помощью [параметра `settings.podSecurityStandards.enforcementAction`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-enforcementaction) модуля `admission-policy-engine`;
- для конкретного пространства имён — с помощью лейбла `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>`.

  Пример команды для установки режима `warn` на все поды в пространстве имён `my-namespace`:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
  ```

### Расширение политики

Вы можете расширить политики `baseline` и `restricted` с помощью шаблонов Gatekeeper,
добавив необходимые проверки к уже существующим.

Чтобы расширить политику, выполните следующее:

1. Создайте шаблон проверки с помощью ресурса ConstraintTemplate.
1. Примените созданный шаблон к политике `baseline` или `restricted`.

Пример шаблона для проверки адреса репозитория с образом контейнера:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sallowedrepos
spec:
  crd:
    spec:
      names:
        kind: K8sAllowedRepos
      validation:
        openAPIV3Schema:
          type: object
          properties:
            repos:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.pod_security_standards.extended

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.initContainers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }
```

Пример применения шаблона к политике `restricted`:

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sAllowedRepos
metadata:
  name: prod-repo
spec:
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
    namespaceSelector:
      matchLabels:
        security.deckhouse.io/pod-policy: restricted
  parameters:
    repos:
      - "mycompany.registry.com"
```

В этом примере проверяется адрес репозитория в поле `image`
у всех подов в пространстве имён с лейблом `security.deckhouse.io/pod-policy: restricted`.
Если адрес в поле `image` создаваемого пода начинается не с `mycompany.registry.com`, под создан не будет.

Вспомогательные ресурсы при создании расширенных политик:

- [документация Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/howto/) с информацией о шаблонах и языке политик;
- [библиотека Gatekeeper](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general) с примерами шаблонов проверок.

## Операционные политики

DVP предоставляет механизм создания операционных политик с помощью [ресурса OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy).
В операционных политиках задаются требования к объектам в кластере:
допустимые репозитории, требуемые ресурсы, наличие проб и т. д.

Команда разработки DVP рекомендует установить следующую политику с минимально необходимым набором требований:

```yaml
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

Эта политика задаёт базовые операционные требования к объектам в кластере,
включая разрешённые container registries контейнеров, обязательные ресурсы и пробы, запрет на использование образов с тегом `latest`,
допустимые классы приоритетов и другие настройки, повышающие безопасность и стабильность работы приложений.

Чтобы назначить данную операционную политику,
примените лейбл `operation-policy.deckhouse.io/enabled=true` к необходимому пространству имён:

```shell
d8 k label ns my-namespace operation-policy.deckhouse.io/enabled=true
```

## Политики безопасности

Используя [ресурс SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy),
вы можете создавать политики безопасности, задающие ограничения на поведение контейнеров в кластере:
доступ к host-сетям, привилегии, использование AppArmor и т. д.

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
          enforce: mypolicy
```

Чтобы назначить данную политику безопасности,
примените лейбл `enforce: "mypolicy"` к необходимому пространству имён.

### Частичное применение политик

Чтобы применить отдельные политики безопасности, не отключая весь предустановленный набор, выполните следующие шаги:

1. Добавьте в необходимое пространство имён лейбл `security.deckhouse.io/pod-policy: privileged`,
   чтобы отключить встроенный набор политик.
1. Создайте [ресурс SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy), соответствующий уровню `baseline` или `restricted`.
   В секции `policies` укажите только необходимые вам настройки безопасности.
1. Добавьте в пространство имён дополнительный лейбл, соответствующий селектору `namespaceSelector` в SecurityPolicy.

Пример конфигурации SecurityPolicy, соответствующий уровню `baseline`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: baseline
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: false
    allowHostNetwork: false
    allowHostPID: false
    allowPrivilegeEscalation: true
    allowPrivileged: false
    allowedAppArmor:
      - runtime/default
      - localhost/*
    allowedCapabilities:
      - AUDIT_WRITE
      - CHOWN
      - DAC_OVERRIDE
      - FOWNER
      - FSETID
      - KILL
      - MKNOD
      - NET_BIND_SERVICE
      - SETFCAP
      - SETGID
      - SETPCAP
      - SETUID
      - SYS_CHROOT
    allowedHostPaths: []
    allowedHostPorts:
      - max: 0
        min: 0
    allowedProcMount: Default
    allowedUnsafeSysctls:
      - kernel.shm_rmid_forced
      - net.ipv4.ip_local_port_range
      - net.ipv4.ip_unprivileged_port_start
      - net.ipv4.tcp_syncookies
      - net.ipv4.ping_group_range
      - net.ipv4.ip_local_reserved_ports
      - net.ipv4.tcp_keepalive_time
      - net.ipv4.tcp_fin_timeout
      - net.ipv4.tcp_keepalive_intvl
      - net.ipv4.tcp_keepalive_probes
    seLinux:
      - type: ""
      - type: container_t
      - type: container_init_t
      - type: container_kvm_t
      - type: container_engine_t
    seccompProfiles:
      allowedProfiles:
        - RuntimeDefault
        - Localhost
        - undefined
        - ''
      allowedLocalhostFiles:
        - '*'
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/baseline-enabled: "true"
```

Пример конфигурации SecurityPolicy, соответствующий уровню `restricted`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: restricted
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: false
    allowHostNetwork: false
    allowHostPID: false
    allowPrivilegeEscalation: false
    allowPrivileged: false
    allowedAppArmor:
      - runtime/default
      - localhost/*
    allowedCapabilities:
      - NET_BIND_SERVICE
    allowedHostPaths: []
    allowedHostPorts:
      - max: 0
        min: 0
    allowedProcMount: Default
    allowedUnsafeSysctls:
      - kernel.shm_rmid_forced
      - net.ipv4.ip_local_port_range
      - net.ipv4.ip_unprivileged_port_start
      - net.ipv4.tcp_syncookies
      - net.ipv4.ping_group_range
      - net.ipv4.ip_local_reserved_ports
      - net.ipv4.tcp_keepalive_time
      - net.ipv4.tcp_fin_timeout
      - net.ipv4.tcp_keepalive_intvl
      - net.ipv4.tcp_keepalive_probes
    allowedVolumes:
      - configMap
      - csi
      - downwardAPI
      - emptyDir
      - ephemeral
      - persistentVolumeClaim
      - projected
      - secret
    requiredDropCapabilities:
      - ALL
    runAsUser:
      rule: MustRunAsNonRoot
    seLinux:
      - type: ""
      - type: container_t
      - type: container_init_t
      - type: container_kvm_t
      - type: container_engine_t
    seccompProfiles:
      allowedProfiles:
        - RuntimeDefault
        - Localhost
      allowedLocalhostFiles:
        - '*'
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/restricted-enabled: "true"
```

## Кастомные ресурсы Gatekeeper

Gatekeeper предоставляет расширенные возможности для модификации ресурсов Kubernetes
с помощью настраиваемых политик (mutation policies).
Эти политики описываются через следующие кастомные ресурсы:

- [AssignMetadata](/modules/admission-policy-engine/gatekeeper-cr.html#assignmetadata) — для изменения секции `metadata` в ресурсе;
- [Assign](/modules/admission-policy-engine/gatekeeper-cr.html#assign) — для изменения других полей, кроме `metadata`;
- [ModifySet](/modules/admission-policy-engine/gatekeeper-cr.html#modifyset) — для добавления или удаления значений из списка, например, аргументов для запуска контейнера;
- [AssignImage](/modules/admission-policy-engine/gatekeeper-cr.html#assignimage) — для изменения параметра `image` ресурса.

Подробнее о возможности изменения ресурсов Kubernetes с помощью настраиваемых политик
можно прочитать [в документации Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).

## Проверка подписи образов

{% alert level="warning" %}
Доступно только в DVP Enterprise edition.
{% endalert %}

DVP поддерживает проверку подписей образов контейнеров с помощью инструмента [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/).
Проверка позволяет убедиться в целостности и подлинности образов.

Чтобы подписать образ с помощью Cosign, выполните следующее:

1. Сгенерируйте пару ключей:

   ```shell
   cosign generate-key-pair
   ```

1. Подпишите образ:

   ```shell
   cosign sign --key <KEY> <IMAGE>
   ```

Чтобы включить проверку подписи образов контейнеров в кластере DVP,
используйте [параметр `policies.verifyImageSignatures`](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) ресурса SecurityPolicy.

Пример конфигурации SecurityPolicy для проверки подписи образов контейнеров:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: verify-image-signatures
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          kubernetes.io/metadata.name: default
  policies:
    verifyImageSignatures:
      - reference: docker.io/myrepo/*
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
      - reference: company.registry.com/*
        dockerCfg: zxc==
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
```

Согласно данной политике, если адрес какого-либо образа контейнера совпадает со значением параметра `reference`
и образ не подписан или подпись не соответствует указанным ключам, создание пода будет запрещено.

Пример вывода ошибки при создании пода с образом контейнера, не прошедшим проверку подписи:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

## Использование альтернатив для управления политиками безопасности

Если вместо встроенного механизма управления политиками безопасности
в кластере DVP используется альтернативное решение (например, [Kyverno](https://kyverno.io/docs/introduction/)),
настройте исключения для следующих пространств имён:

- `kube-system`;
- все пространства имён с префиксом `d8-*` (например, `d8-system`).

Без этих исключений политики могут блокировать или нарушать работу системных компонентов.
