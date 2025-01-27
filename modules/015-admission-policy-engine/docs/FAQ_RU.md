---
title: "Модуль admission-policy-engine: FAQ"
---

## Как настроить альтернативные решения по управлению политиками безопасности?

Для корректной работы DKP необходимы расширенные привилегии на запуск и работу полезной нагрузки системных компонентов. Если вместо модуля admission-policy-engine используется альтернативное решение по управлению политиками безопасности (например, Kyverno), необходима настройка исключений для следующих пространств имен:
- `kube-system`;
- все пространства имен с префиксом `d8-*` (например, `d8-system`).

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

Подробнее о шаблонах и языке политик можно узнать в документации Gatekeeper.

Больше примеров описания проверок для расширения политики можно найти в библиотеке Gatekeeper.

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

## Проверка подписи образов

В модуле реализована функция проверки подписи образов контейнеров, подписанных с помощью инструмента Cosign. Проверка подписи образов контейнеров позволяет убедиться в их целостности (что образ не был изменен после его создания) и подлинности (что образ был создан доверенным источником). Включить проверку подписи образов контейнеров в кластере можно с помощью параметра [policies.verifyImageSignatures](cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) ресурса SecurityPolicy.

{% offtopic title="Как подписать образ..." %}
Шаги для подписания образа:
- Сгенерируйте ключи: `cosign generate-key-pair`
- Подпишите образ: `cosign sign --key <key> <image>`

Подробнее о работе с Cosign можно узнать в документации.
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
