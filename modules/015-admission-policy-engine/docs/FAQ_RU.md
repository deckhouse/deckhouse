---
title: "Модуль admission-policy-engine: FAQ"
---

## Как настроить альтернативные решения по управлению политиками безопасности?

Для корректной работы DKP необходимы расширенные привилегии на запуск и работу полезной нагрузки системных компонентов. Если вместо модуля admission-policy-engine используется альтернативное решение по управлению политиками безопасности (например, Kyverno), необходима настройка исключений для следующих пространств имен:

- `kube-system`;
- все пространства имен с префиксом `d8-*` (например, `d8-system`).

## Как расширить политики Pod Security Standards?

{% alert level="info" %}
Pod Security Standards реагируют на label `security.deckhouse.io/pod-policy: restricted` или `security.deckhouse.io/pod-policy: baseline`.
{% endalert %}

Чтобы расширить политику Pod Security Standards, добавив к существующим проверкам политики свои собственные, необходимо:

- создать шаблон проверки (ресурс `ConstraintTemplate`);
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

Пример демонстрирует настройку проверки адреса репозитория в поле `image` у всех подов, создающихся в пространстве имен, имеющих label `security.deckhouse.io/pod-policy: restricted`. Если адрес в поле `image` создаваемого пода начинается не с `mycompany.registry.com`, под создан не будет.

Подробнее о шаблонах и языке политик можно узнать [в документации Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/howto/).

Больше примеров описания проверок для расширения политики можно найти [в библиотеке Gatekeeper](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general).

## Как включить одну или несколько политик Pod Security Standards, не отключая весь набор?

Чтобы применить только нужные политики безопасности, не отключая весь предустановленный набор:

1. Добавьте в нужное пространство имён метку: `security.deckhouse.io/pod-policy: privileged`, чтобы отключить встроенный набор политик.
1. Создайте ресурс SecurityPolicy, соответствующий уровню [baseline](https://kubernetes.io/docs/concepts/security/pod-security-standards/#baseline) или [restricted](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted). В секции `policies` укажите только необходимые вам настройки.
1. Добавьте в пространство имён дополнительную метку, которая будет соответствовать селектору `namespaceSelector` в SecurityPolicy. В примерах ниже это `security-policy.deckhouse.io/baseline-enabled: "true"` либо `security-policy.deckhouse.io/restricted-enabled: "true"`

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

{% alert level="warning" %}Доступно в следующих редакциях: SE+, EE, CSE Lite (1.67), CSE Pro (1.67).{% endalert %}

В модуле реализована функция проверки подписи образов контейнеров, подписанных с помощью инструмента [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/#:~:text=To%20generate%20a%20key%20pair,prompted%20to%20provide%20a%20password.&text=Alternatively%2C%20you%20can%20use%20the,%2C%20ECDSA%2C%20and%20ED25519%20keys). Проверка подписи образов контейнеров позволяет убедиться в их целостности (что образ не был изменен после его создания) и подлинности (что образ был создан доверенным источником). Включить проверку подписи образов контейнеров в кластере можно с помощью параметра [policies.verifyImageSignatures](cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) ресурса SecurityPolicy.

{% offtopic title="Как подписать образ..." %}
Шаги для подписания образа:

- Сгенерируйте ключи: `cosign generate-key-pair`
- Подпишите образ: `cosign sign --key <key> <image>`

Подробнее о работе с Cosign можно узнать в [документации](https://docs.sigstore.dev/cosign/key_management).
{% endofftopic %}

Пример SecurityPolicy для настройки проверки подписи образов контейнеров:

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

Политика не влияет на создание подов, адреса образов контейнеров которых не подходят под описанные в параметре `reference`.  Если же адрес какого-либо образа контейнера подходит под описанные в параметре `reference` политики, и образ не подписан или подпись не соответствует указанным в политике ключам, создание пода будет запрещено.

Пример вывода ошибки при создании пода с образом контейнера, не прошедшим проверку подписи:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

## Как запретить удаление узла без метки

> Примечание. Операции DELETE обрабатываются Gatekeeper по умолчанию.

Можно создать собственную политику Gatekeeper, запрещающую удаление узла без специальной метки. Пример ниже использует `oldObject` для проверки меток удаляемого узла:

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

## Как запретить exec/attach в определённые поды

> Примечание. Вебхук модуля `admission-policy-engine` направляет запросы `CONNECT` для `pods/exec` и `pods/attach` через Gatekeeper. Это позволяет создавать пользовательские политики для валидации или запрета операций `kubectl exec` / `kubectl attach`.

### Встроенная политика для подов с heritage: deckhouse

Модуль включает встроенную политику `D8DenyExecHeritage`, которая запрещает exec/attach во все поды с меткой `heritage: deckhouse`. Это защищает системные компоненты, управляемые Deckhouse.

Исключения (эти пользователи могут выполнять exec в heritage-поды):
- `system:sudouser`
- Сервисные аккаунты из пространств имён `d8-*` (`system:serviceaccount:d8-*`)
- Сервисные аккаунты из пространств имён `kube-*` (`system:serviceaccount:kube-*`)

### Пример пользовательской политики

Можно создать собственную политику Gatekeeper для запрета exec/attach в определённых пространствах имён. Пример ниже использует `input.review.operation` и `input.review.resource.resource` для проверки операций CONNECT:

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

        is_connect { input.review.operation == "CONNECT" }
        is_exec_or_attach { input.review.resource.resource == "pods/exec" }
        is_exec_or_attach { input.review.resource.resource == "pods/attach" }

        is_forbidden_namespace {
          ns := input.review.namespace
          ns == input.parameters.forbiddenNamespaces[_]
        }

        violation[{"msg": msg}] {
          is_connect
          is_exec_or_attach
          is_forbidden_namespace
          msg := sprintf("Exec/attach запрещён в пространстве имён %q", [input.review.namespace])
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

Ключевые моменты для валидации CONNECT:
- Используйте `input.review.operation == "CONNECT"` для проверки операций CONNECT.
- Используйте `input.review.resource.resource` для проверки субресурсов `pods/exec` или `pods/attach`.
- Информация о пользователе доступна в `input.review.userInfo.username` и `input.review.userInfo.groups`.
- Пространство имён доступно в `input.review.namespace`.
