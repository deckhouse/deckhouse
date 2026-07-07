---
title: "Модуль admission-policy-engine: FAQ"
description: "Ответы на часто задаваемые вопросы о модуле admission-policy-engine."
---

## Как настроить альтернативные решения по управлению политиками безопасности?

Для корректной работы DKP необходимы расширенные привилегии на запуск и работу полезной нагрузки системных компонентов. Если вместо модуля admission-policy-engine используется альтернативное решение по управлению политиками безопасности (например, Kyverno), необходима настройка исключений для следующих неймспейсов:

- `kube-system`;
- все неймспейсы с префиксом `d8-*` (например, `d8-system`).

## Как настроить селекторы политик?

В `OperationPolicy` и `SecurityPolicy` поле `spec.match` определяет, на какие именно объекты (поды) в кластере будет распространяться политика. Оно обязательно должно присутствовать в конфигурации. Фильтрация выполняется путём комбинирования двух основных критериев: селектора подов (`labelSelector`) и селектора неймспейсов (`namespaceSelector`).

Если указаны оба селектора, политика применяется только к тем подам, которые одновременно:

- удовлетворяют условиям выбора подов;
- находятся в неймспейсах, прошедших фильтрацию.

Если какой-либо из селекторов не указан, соответствующая проверка не производится (будут использоваться все поды или неймспейсы).

- `spec.match.labelSelector` – выбор подов

С помощью `labelSelector` задаются критерии отбора подов по их лейблам. Поддерживаются два взаимоисключающих способа:

- `matchLabels` – простая проверка на точное совпадение лейблов (ключ‑значение). Под должен иметь все указанные лейблы.
- `matchExpressions` – гибкие выражения с операторами. Каждое выражение задаётся объектом с полями:
  - `key` (строка, обязательно) – имя лейбла.
  - `operator` (строка, обязательно) – одно из значений: `In`, `NotIn`, `Exists`, `DoesNotExist`.
  - `values` (массив строк) – список значений для операторов `In` / `NotIn`; для `Exists` / `DoesNotExist` не указывается.

Все элементы списка `matchExpressions` объединяются логическим И – под должен удовлетворять каждому выражению.

Примеры:

```yaml
spec:
  match:
    labelSelector:
      matchLabels:
        app: nginx
        role: frontend
```

```yaml
spec:
  match:
    labelSelector:
      matchExpressions:
        - key: tier
          operator: In
          values:
            - production
            - staging
        - key: monitoring
          operator: Exists
```

- `spec.match.namespaceSelector` – выбор неймспейсов

Позволяет ограничить неймспейсы, в которых действует политика. Внутри можно использовать три фильтра:

- `matchNames` – явный список разрешённых неймспейсов. Если задан, политика действует только в перечисленных неймспейсах.
- `excludeNames` – список исключаемых неймспейсов. Политика будет действовать во всех неймспейсах, кроме указанных.
- `labelSelector` – селектор по лейблам самого объекта Namespace.

Если задано несколько полей, они объединяются логическим И:

1. берётся множество из `matchNames` (или все неймспейсы, если `matchNames` не задан);
1. применяется `labelSelector` (если задан);
1. вычитаются `excludeNames`.

Формула:

`result = (base_from_matchNames ∩ selected_by_labelSelector) \ excludeNames`

Рекомендуется не смешивать `matchNames`, `excludeNames`, `labelSelector` без явной необходимости.

### Когда лучше использовать `labelSelector`

Используйте `labelSelector`, когда политика должна автоматически применяться к группе неймспейсов по признаку, а не по фиксированным именам.

Например:

- «все неймспейсы с лейблом `env=prod`»;
- «все неймспейсы команды `team=backend`» (с соответствующим лейблом);
- «все неймспейсы с `security.deckhouse.io/pod-policy=restricted`».

`labelSelector` особенно полезен, когда неймспейсы создаются/удаляются динамически: достаточно автоматически проставлять label на неймспейс при создании, и политика начнёт действовать без редактирования политики.

`labelSelector` не обязателен, если у вас небольшой статичный список неймспейсов – тогда проще и легче читается подход с использованием `matchNames`.

### Можно ли вместе использовать `matchNames` и `labelSelector`

Да, технически они не взаимоисключающие: их можно указывать вместе, и тогда сработает пересечение.

Но на практике это часто ухудшает читаемость и усложняет сопровождение. Поэтому рекомендуется выбирать один основной способ отбора:

- либо `matchNames + excludeNames`;
- либо `labelSelector + excludeNames`.

Так проще понять, почему конкретный неймспейс попал/не попал под политику.

### Типовые сценарии

1. Статичный список окружений → `matchNames + excludeNames`.
1. Динамические окружения/команды → `labelSelector + excludeNames`.
1. Комбинация `matchNames + labelSelector` – только если действительно нужно пересечение двух независимых условий.

Примеры:

Статичный список неймспейсов (`matchNames + excludeNames`)

```yaml
spec:
  match:
    namespaceSelector:
      matchNames:
        - production
        - staging
      excludeNames:
        - staging
```

Итог: политика действует только в неймспейсе `production`.

Динамический выбор по лейблам (`labelSelector + excludeNames`)

```yaml
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          team: backend
          environment: production
      excludeNames:
        - backend-sandbox
```

Итог: все неймспейсы с лейблами `team=backend` и `environment=production`, кроме `backend-sandbox`.

Гибкая фильтрация по выражениям (`labelSelector.matchExpressions`)

```yaml
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchExpressions:
          - key: compliance
            operator: In
            values:
              - pci
              - sox
          - key: lifecycle
            operator: NotIn
            values:
              - deprecated
```

Итог: только неймспейсы с лейблами `compliance=pci` или `compliance=sox` и без лейбла `lifecycle=deprecated`.

Комбинация `matchNames` и `labelSelector` (пересечение, использовать осторожно)

```yaml
spec:
  match:
    namespaceSelector:
      matchNames:
        - production
        - staging
        - qa
      labelSelector:
        matchLabels:
          team: backend
```

Итог: применится только к неймспейсам, которые одновременно:

- входят в список `production|staging|qa`;
- имеют лейбл `team=backend`.

Если, например, `qa` не имеет `team=backend`, он не попадёт под политику.

## Как расширить политики Pod Security Standards?

{% alert level="info" %}
Pod Security Standards реагируют на label `security.deckhouse.io/pod-policy: restricted` или `security.deckhouse.io/pod-policy: baseline`.
{% endalert %}

Чтобы расширить политику Pod Security Standards, добавив к существующим проверкам политики свои собственные, необходимо:

- создать шаблон проверки (`ConstraintTemplate`);
- привязать его к политике `restricted` или `baseline`.

Пример шаблона для проверки адреса репозитория образа контейнера:

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

Пример привязки проверки к политике `restricted`:

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

Пример демонстрирует настройку проверки адреса репозитория в поле `image` у всех подов, создающихся в неймспейсах, имеющих label `security.deckhouse.io/pod-policy: restricted`. Если адрес в поле `image` создаваемого пода начинается не с `mycompany.registry.com`, под создан не будет.

Подробнее о шаблонах и языке политик можно узнать [в документации Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/howto/).

Больше примеров описания проверок для расширения политики можно найти [в библиотеке Gatekeeper](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general).

## Как включить одну или несколько политик Pod Security Standards, не отключая весь набор?

Чтобы применить только нужные политики безопасности, не отключая весь предустановленный набор:

1. Добавьте в нужный неймспейс лейбл: `security.deckhouse.io/pod-policy: privileged`, чтобы отключить встроенный набор политик.
1. Создайте ресурс SecurityPolicy, соответствующий уровню [baseline](https://kubernetes.io/docs/concepts/security/pod-security-standards/#baseline) или [restricted](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted). В секции `policies` укажите только необходимые вам настройки.
1. Добавьте в неймспейс дополнительный лейбл, который будет соответствовать селектору `namespaceSelector` в SecurityPolicy. В примерах ниже это `security-policy.deckhouse.io/baseline-enabled: "true"` либо `security-policy.deckhouse.io/restricted-enabled: "true"`.

SecurityPolicy, соответствующая baseline:

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
          security-policy.deckhouse.io/baseline-enabled: "true"
```

SecurityPolicy, соответствующая restricted:

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
          security-policy.deckhouse.io/restricted-enabled: "true"
```

## Что, если несколько политик (операционных или безопасности) применяются на один объект?

В этом случае необходимо, чтобы конфигурация объекта соответствовала всем политикам, которые на него распространяются.

Например, рассмотрим две следующие политики безопасности:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - MKNOD
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: bar
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    requiredDropCapabilities:
    - NET_BIND_SERVICE
```

Тогда для выполнения требований приведенных политик безопасности в спецификации контейнера нужно указать:

```yaml
    securityContext:
      capabilities:
        drop:
          - MKNOD
          - NET_BIND_SERVICE
      readOnlyRootFilesystem: true
```

## Проверка подписи образов

{% alert level="warning" %}
Доступно в следующих редакциях DKP: SE+, EE, CSE Lite, CSE Pro.

Поддерживается Cosign не выше v2. Версии v3 и выше не поддерживаются.
{% endalert %}

В модуле реализована функция проверки подписи образов контейнеров, подписанных с помощью инструмента [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/#:~:text=To%20generate%20a%20key%20pair,prompted%20to%20provide%20a%20password.&text=Alternatively%2C%20you%20can%20use%20the,%2C%20ECDSA%2C%20and%20ED25519%20keys). Подробнее о подписании и проверке образов контейнеров можно узнать в [документации DKP](/products/kubernetes-platform/documentation/v1/admin/configuration/security/policies.html#проверка-подписи-образов).

## Как запретить удаление узла без лейбла

{% alert level="info" %}
Операции DELETE обрабатываются Gatekeeper по умолчанию.
{% endalert %}

Можно создать собственную политику Gatekeeper, запрещающую удаление узла без специального лейбла. Пример ниже использует `oldObject` для проверки лейблов удаляемого узла:

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
          msg := sprintf("Удаление Node запрещено. Добавьте лейбл %q=%q.", [input.parameters.requiredLabelKey, input.parameters.requiredLabelValue])
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

## Как запретить операции kubectl exec и kubectl attach в определённые поды?

Вебхук модуля `admission-policy-engine` направляет запросы `CONNECT` для `pods/exec` и `pods/attach` через Gatekeeper. Это позволяет создавать пользовательские политики для разрешения или запрета операций `kubectl exec` и `kubectl attach`.

### Встроенная политика для подов с heritage: deckhouse

Для защиты системных компонентов под управлением Deckhouse в модуле `admission-policy-engine` предусмотрена встроенная политика `D8DenyExecHeritage`, которая запрещает выполнение операций `kubectl exec` и `kubectl attach` во все поды с лейблом `heritage: deckhouse`.

Политика не распространяется на следующих пользователей, которым разрешены операции `kubectl exec` и `kubectl attach` в поды с лейблом `heritage: deckhouse`:

- `system:sudouser`;
- сервисные аккаунты из неймспейсов `d8-*` (`system:serviceaccount:d8-*`);
- сервисные аккаунты из неймспейсов `kube-*` (`system:serviceaccount:kube-*`).

### Встроенная политика для финалайзеров Deckhouse

Для защиты ресурсов, управляемых контроллерами Deckhouse, в модуле `admission-policy-engine` предусмотрена встроенная ValidatingAdmissionPolicy `deny-deckhouse-finalizers.deckhouse.io`, которая запрещает удалять финалайзеры, содержащие подстроку `.deckhouse.io/`, на любых ресурсах кластера.

Политика не распространяется на следующих пользователей, которым разрешено снимать такие финалайзеры:

- системные контроллеры Kubernetes (`system:kube-controller-manager`, `system:kube-scheduler` и др.);
- `system:sudouser`, `dhctl`, `observability`;
- сервисные аккаунты из неймспейсов `d8-*` (`system:serviceaccount:d8-*`);
- сервисные аккаунты из неймспейсов `kube-*` (`system:serviceaccount:kube-*`).

### Пример пользовательской политики

Вы можете создать собственную политику Gatekeeper для запрета операций `kubectl exec` и `kubectl attach` в определённых неймспейсах. В примере ниже используются `input.review.operation` и `input.review.resource.resource` для проверки операций `CONNECT`:

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
