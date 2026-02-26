---
title: Политики безопасности
permalink: ru/admin/configuration/security/policies.html
description: "Настройка политик безопасности в Deckhouse Kubernetes Platform с использованием Gatekeeper и Pod Security Standards. Применение политик, соответствие требованиям и управление безопасностью кластера."
lang: ru
search: security policies, pod security standards, gatekeeper, security enforcement, policy management, политики безопасности
---

Deckhouse Kubernetes Platform (DKP) позволяет управлять безопасностью приложений в кластере с помощью набора политик,
соответствующих модели [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) и дополнительно расширяемых через встроенные механизмы DKP.

Для реализации политик безопасности в DKP используется [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

## Применение Pod Security Standards

В DKP поддерживаются три уровня политики безопасности:

- `privileged` — неограничивающая политика с максимально широким уровнем разрешений;
- `baseline` — минимально ограничивающая политика,
  которая предотвращает наиболее известные и популярные способы повышения привилегий.
  Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `restricted` — политика со значительными ограничениями. Предъявляет самые жесткие требования к подам.

### Политика по умолчанию

Используемая по умолчанию политика определяется следующим образом:

- в версиях DKP до v1.55 политика по умолчанию — `privileged`;
- начиная с версии DKP v1.55, политика по умолчанию — `baseline`.

{% alert level="info" %}
При обновлении DKP на версию v1.55 или выше политика по умолчанию не изменится автоматически.
{% endalert %}

### Назначение политики

Варианты назначения политики:

- глобально — с помощью [параметра `settings.podSecurityStandards.defaultPolicy`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) модуля [`admission-policy-engine`](/modules/admission-policy-engine/);
- для конкретного неймспейса — с помощью лейбла `security.deckhouse.io/pod-policy=<POLICY_NAME>`.

  Пример команды для назначения политики `restricted` на все поды в неймспейсе `my-namespace`:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
  ```

### Режимы применения политики

Допустимые режимы применения политик:

- `deny` — запрещает выполнений действий.
- `dryrun` — не влияет на выполнение действий и используется для отладки. Информацию о событиях можно посмотреть в Grafana или в консоли с помощью команды `d8 k` или `kubectl`.
- `warn` — работает как `dryrun`, но дополнительно выводит предупреждение с указанием причины,
  по которой бы произошёл запрет действия в режиме `deny`.

По умолчанию, политики Pod Security Standards в DKP применяются в режиме `deny`.
В этом режиме поды приложений, не удовлетворяющие политикам, не могут быть запущены в кластере.

Как и в случае с назначением политик, режим их применения можно задать:

- глобально — с помощью [параметра `settings.podSecurityStandards.enforcementAction`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-enforcementaction) модуля [`admission-policy-engine`](/modules/admission-policy-engine/);
- для конкретного неймспейса — с помощью лейбла `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>`.

  Пример команды для установки режима `warn` на все поды в неймспейсе `my-namespace`:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
  ```

### Расширение политики

Вы можете расширить политики `baseline` и `restricted` с помощью шаблонов Gatekeeper,
добавив необходимые проверки к уже существующим.

Чтобы расширить политику, выполните следующее:

1. Создайте шаблон проверки с помощью ConstraintTemplate.
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
у всех подов в неймспейсе с лейблом `security.deckhouse.io/pod-policy: restricted`.
Если адрес в поле `image` создаваемого пода начинается не с `mycompany.registry.com`, под создан не будет.

Вспомогательные ресурсы при создании расширенных политик:

- [примеры пользовательских политик](#примеры-пользовательских-политик-gatekeeper);
- [документация Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/howto/) с информацией о шаблонах и языке политик;
- [библиотека Gatekeeper](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general) с примерами шаблонов проверок.

## Операционные политики

DKP предоставляет механизм создания операционных политик с помощью [OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy).
В операционных политиках задаются требования к объектам в кластере:
допустимые репозитории, требуемые ресурсы, наличие проб и т. д.

Команда разработки DKP рекомендует установить следующую политику с минимально необходимым набором требований:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
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
          operation-policy.deckhouse.io/enabled: "true"
```

Эта политика задаёт базовые операционные требования к объектам в кластере,
включая разрешённые container registries контейнеров, обязательные ресурсы и пробы, запрет на использование образов с тегом `latest`,
допустимые классы приоритетов и другие настройки, повышающие безопасность и стабильность работы приложений.

Чтобы назначить данную операционную политику,
примените лейбл `operation-policy.deckhouse.io/enabled=true` к необходимому неймспейсу:

```shell
d8 k label ns my-namespace operation-policy.deckhouse.io/enabled=true
```

## Политики безопасности

Используя [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy),
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

Чтобы назначить данную политику безопасности, примените лейбл `enforce: "mypolicy"` к необходимому неймспейсу.

### Частичное применение политик

Чтобы применить отдельные политики безопасности, не отключая весь предустановленный набор, выполните следующие шаги:

1. Добавьте на необходимый неймспейс лейбл `security.deckhouse.io/pod-policy: privileged`, чтобы отключить встроенный набор политик.
1. Создайте [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy), соответствующий уровню `baseline` или `restricted`.
   В секции `policies` укажите только необходимые вам настройки безопасности.
1. Добавьте на неймспейс дополнительный лейбл, соответствующий селектору `namespaceSelector` в SecurityPolicy.

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

## Примеры пользовательских политик Gatekeeper

Здесь приведены примеры политик Gatekeeper, с помощью которых вы можете расширить стандартные механизмы безопасности кластера.

### Запрет на удаление узла без указанного лейбла

{% alert level="info" %}
Операции `DELETE` обрабатываются Gatekeeper по умолчанию.
{% endalert %}

Вы можете создать политику Gatekeeper, запрещающую удаление узла без специального лейбла.

В примере ниже используется поле `oldObject` для проверки лейблов удаляемого узла:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8customnodedeleteguard
spec:
  crd:
    spec:
      names:
        kind: D8CustomNodeDeleteGuard
      validation:
        openAPIV3Schema:
          type: object
          properties:
            requiredLabelKey:
              type: string
            requiredLabelValue:
              type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.custom

        is_delete { input.review.operation == "DELETE" }
        is_node { input.review.kind.kind == "Node" }

        has_required_label {
          key := input.parameters.requiredLabelKey
          val := input.parameters.requiredLabelValue
          obj := input.review.oldObject
          obj.metadata.labels[key] == val
        }

        violation[{"msg": msg}] {
          is_delete
          is_node
          not has_required_label
          msg := sprintf("Удаление Node запрещено. Добавьте метку %q=%q.", [input.parameters.requiredLabelKey, input.parameters.requiredLabelValue])
        }
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: D8CustomNodeDeleteGuard
metadata:
  name: require-node-delete-label
spec:
  enforcementAction: warn
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Node"]
  parameters:
    requiredLabelKey: "admission.deckhouse.io/allow-delete"
    requiredLabelValue: "true"
```

### Запрет на выполнение операций exec и attach в определённые поды

Вебхук модуля `admission-policy-engine` направляет запросы `CONNECT` для `pods/exec` и `pods/attach` через Gatekeeper.
Это позволяет создавать пользовательские политики для разрешения или запрета операций `kubectl exec` и `kubectl attach`.

#### Встроенная политика для подов с лейблом `heritage: deckhouse`

Для защиты системных компонентов под управлением Deckhouse
в модуле `admission-policy-engine` предусмотрена встроенная политика `D8DenyExecHeritage`,
которая запрещает выполнение операций `kubectl exec` и `kubectl attach` во все поды с лейблом `heritage: deckhouse`.

Политика не распространяется на следующих пользователей,
которым разрешены операции `kubectl exec` и `kubectl attach` в поды с лейблом `heritage: deckhouse`:

- `system:sudouser`;
- сервисные аккаунты из неймспейсов `d8-*` (`system:serviceaccount:d8-*`);
- сервисные аккаунты из неймспейсов `kube-*` (`system:serviceaccount:kube-*`).

#### Пример пользовательской политики

Вы можете создать собственную политику Gatekeeper для запрета операций `kubectl exec` и `kubectl attach` в определённых неймспейсах.
В примере ниже используются `input.review.operation` и `input.review.resource.resource` для проверки операций `CONNECT`:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8customdenyexec
spec:
  crd:
    spec:
      names:
        kind: D8CustomDenyExec
      validation:
        openAPIV3Schema:
          type: object
          properties:
            forbiddenNamespaces:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.custom

        is_connect {
          input.review.operation == "CONNECT"
        }

        # requestSubResource предпочтительнее, но на всякий случай падаем в subResource
        subresource_is(sub) {
          sr := object.get(input.review, "requestSubResource", input.review.subResource)
          sr == sub
        }

        is_exec_or_attach {
          input.review.resource.resource == "pods"
          subresource_is("exec")
        }

        is_exec_or_attach {
          input.review.resource.resource == "pods"
          subresource_is("attach")
        }

        is_forbidden_namespace {
          ns := input.review.namespace
          ns == input.parameters.forbiddenNamespaces[_]
        }

        violation[{"msg": msg}] {
          is_connect
          is_exec_or_attach
          is_forbidden_namespace
          msg := sprintf("Exec/attach запрещён в неймспейсе %q", [input.review.namespace])
        }
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: D8CustomDenyExec
metadata:
  name: deny-exec-in-namespaces
spec:
  enforcementAction: deny
  match:
    kinds:
      - apiGroups: ["*"]
        kinds: ["*"]
    scope: Namespaced
  parameters:
    forbiddenNamespaces:
      - production
      - staging
```

Ключевые данные и проверки, доступные при валидации операций `CONNECT`:

- Используйте `input.review.operation == "CONNECT"` для проверки операций `CONNECT`.
- Информация о пользователе доступна в `input.review.userInfo.username` и `input.review.userInfo.groups`.
- Неймспейс доступен в `input.review.namespace`.

## Проверка подписи образов

{% alert level="warning" %}
Доступно в следующих редакциях DKP: SE+, EE, CSE Lite (1.67), CSE Pro (1.67).

Поддерживается Cosign не выше v2. Версии v3 и выше не поддерживаются.
{% endalert %}

DKP поддерживает проверку подписей образов контейнеров с помощью инструмента [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/).  
Проверка подписи образов контейнеров позволяет убедиться в их целостности (что образ не был изменен после его создания) и подлинности (что образ был создан доверенным источником). Включить проверку подписи образов контейнеров в кластере можно с помощью параметра [policies.verifyImageSignatures](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) SecurityPolicy.  

При подписании образа, в хранилище образов контейнеров создается специальный тег, который содержит подпись образа.  
Подпись формируется для хеш-суммы образа (image digest). Например, для образа `my-repo/app:latest` с хешем `sha256:abc123EXAMPLE`, в хранилище образов появится образ `my-repo/app:sha256-abc123EXAMPLE.sig`. Таким образом, процесс подписи образа заключается в расчете и публикации дополнительного тега в хранилище образов контейнеров, без изменения существующего образа.

После подписания образа повторная его загрузка (push) в хранилище не требуется. Необходимо только авторизоваться в хранилище образов контейнеров с правами на запись.

Чтобы подписать образ с помощью Cosign, выполните следующее:

1. Убедитесь что используется Cosign версии 2 или ниже.

   Проверить версию: `cosign version`.

   ```shell
   cosign version
   ```

1. Сгенерируйте пару ключей (публичный и приватный):

   ```shell
   cosign generate-key-pair
   ```

1. Подпишите образ в хранилище образов контейнеров с помощью сгенерированного приватного ключа:

   ```shell
   cosign sign --key <KEY> <REGISTRY_IMAGE_PATH>
   ```

   Здесь:
   - `<REGISTRY_IMAGE_PATH>` — путь к образу, который нужно указать при запуске, например: `registry.private.ru/labs/application/image:latest`.

Чтобы включить проверку подписи образов контейнеров в кластере DKP:

1. Используйте [параметр `policies.verifyImageSignatures`](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) SecurityPolicy, указав сгенерированный публичный ключ.

   Пример конфигурации SecurityPolicy для проверки подписи образов контейнеров в хранилище `registry.private.ru`, размещенные по пути `/labs/application/`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: SecurityPolicy
   metadata:
     name: verify-image-test
   spec:
     enforcementAction: Deny
     match:
       namespaceSelector:
         labelSelector:
           matchLabels:
             kubernetes.io/metadata.name: test-namespace
     policies:
       allowHostIPC: true
       allowHostNetwork: true
       allowHostPID: false
       allowPrivilegeEscalation: true
       allowPrivileged: false
       allowRbacWildcards: true
       verifyImageSignatures:
         - publicKeys:
             - |-
               -----BEGIN PUBLIC KEY-----
               ...
               -----END PUBLIC KEY-----
           reference: registry.private.ru/labs/application/*
   ```

1. Создайте [OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy), ограничивающий запуск подов со сторонних registry:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: OperationPolicy
   metadata:
     name: test-operation-policy
   spec:
    enforcementAction: Deny
    match:
      namespaceSelector:
      labelSelector:
        matchLabels:
           operation-policy.deckhouse.io/enabled: "true"
   policies:
     allowedRepos:
     - registry.private.ru
   ```

1. Добавьте метку на неймспейс, где необходимо включить проверку подписи командой (укажите нужный неймспейс):

   ```shell
   d8 k label ns <NAMESPACE> security.deckhouse.io/verify-image-test=
   ```

1. Для проверки работы механизма подписи образов разверните поды в неймспейсе, с подписанным и неподписанным образами (укажите нужный неймспейс):

   ```shell
   d8 k  -n <NAMESPACE> run signed-pod --image=<ПОДПИСАННЫЙ_ОБРАЗ>
   d8 k  -n <NAMESPACE> run unsigned-pod --image=<НЕПОДПИСАННЫЙ_ОБРАЗ>
   ```

Согласно данной политике, если адрес какого-либо образа контейнера совпадает со значением параметра `reference` и образ не подписан или подпись не соответствует указанным ключам, создание пода будет запрещено.

Пример вывода ошибки при создании пода с образом контейнера, не прошедшим проверку подписи:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

## Использование альтернатив для управления политиками безопасности

Если вместо встроенного механизма управления политиками безопасности
в кластере DKP используется альтернативное решение (например, [Kyverno](https://kyverno.io/docs/introduction/)),
настройте исключения для следующих неймспейсов:

- `kube-system`;
- все неймспейсы с префиксом `d8-*` (например, `d8-system`).

Без этих исключений политики могут блокировать или нарушать работу системных компонентов.
