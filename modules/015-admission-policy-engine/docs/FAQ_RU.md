---
title: "Модуль admission-policy-engine: FAQ"
---

## Как расширить политики Pod Security Standards?

> Pod Security Standards реагируют на label `security.deckhouse.io/pod-policy: restricted` или `security.deckhouse.io/pod-policy: baseline`.

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

## Что, если несколько политик (операционных или безопасности) применяются на один объект?

В таком случае необходимо, чтобы конфигурация объекта соответствовала всем политикам, которые на него распространяются.

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
