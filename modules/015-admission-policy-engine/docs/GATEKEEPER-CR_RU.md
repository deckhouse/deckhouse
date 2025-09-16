---
title: "Модуль admission-policy-engine: Custom Resources (от Gatekeeper)"
---

## Mutation Custom Resources

{% alert level="info" %}
Для мутационных хуков используется настройка `reinvocationPolicy: IfNeeded` в MutatingWebhookConfiguration. Подробнее [в документации Kubernetes.](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#reinvocation-policy)
{% endalert %}  

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#mutation-crds)

Представляют собой набор настраиваемых политик модификации ресурсов Kubernets в момент их создания.

### AssignMetadata

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#assignmetadata)

Позволяет изменять секцию `Metadata` ресурса.  
На данный момент сервисом Gatekeeper разрешено только **добавление** объектов `lables` и `annotations`. Изменение существующих объектов не предусмотрено.

{% alert level="info" %}
В `spec.match.kinds` запрещено использовать `*`. При указании `*` мутация не применяется. Вместо этого необходимо явно перечислять целевые ресурсы (`kinds`) и их `apiGroups`.
{% endalert %}

Пример 1. Добавление лейбла `owner` со значением `admin` во всех пространствах имён:
  
```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: demo-annotation-owner
spec:
  match:
    scope: Namespaced
  location: "metadata.labels.owner"
  parameters:
    assign:
      value: "admin"
```

Пример 2. Добавление лейбла в конкретном пространстве имён и только на выбранные ресурсы:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: set-labels-<your_namespace>
spec:
  match:
    scope: Namespaced
    namespaces: ["<your_namespace>"]
    kinds:
    - apiGroups: [""]
      kinds: ["Pod"] # Запрещено использовать "*" .
  location: "metadata.labels.<your_label_name>"
  parameters:
    assign:
      value: <your_label_value>
```

### Assign

<!-- 
[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#assignmetadata) 
Отдельной ссылки в документации Gatekeeper на данный CR нет
-->

Позволяет изменять поля, за пределом секции `Metadata`.

Пример установки `imagePullPolicy` для всех контейнеров на `Always` во всех пространствах имен, кроме `system`:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: Assign
metadata:
  name: demo-image-pull-policy
spec:
  applyTo:
  - groups: [""]
    kinds: ["Pod"]
    versions: ["v1"]
  match:
    scope: Namespaced
    kinds:
    - apiGroups: ["*"]
      kinds: ["Pod"]
    excludedNamespaces: ["system"]
  location: "spec.containers[name:*].imagePullPolicy"
  parameters:
    assign:
      value: Always
```

### ModifySet

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#modifyset)

Позволяет добавлять и удалять элементы из списка, например из списка аргументов для запуска контейнера.  
Новые значения добавляются в конец списка.

Пример удаления аргумента `--alsologtostderr` из всех контейнеров в поде:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: ModifySet
metadata:
  name: remove-err-logging
spec:
  applyTo:
  - groups: [""]
    kinds: ["Pod"]
    versions: ["v1"]
  location: "spec.containers[name: *].args"
  parameters:
    operation: prune
    values:
      fromList:
        - --alsologtostderr
```

### AssignImage

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#assignimage)

Позволяет вносить изменения в параметр `image` ресурса.

Пример изменения параметра `image` на значение `my.registry.io/repo/app@sha256:abcde67890123456789abc345678901a`:
  
```yaml
apiVersion: mutations.gatekeeper.sh/v1alpha1
kind: AssignImage
metadata:
  name: assign-container-image
spec:
  applyTo:
  - groups: [ "" ]
    kinds: [ "Pod" ]
    versions: [ "v1" ]
  location: "spec.containers[name:*].image"
  parameters:
    assignDomain: "my.registry.io"
    assignPath: "repo/app"
    assignTag: "@sha256:abcde67890123456789abc345678901a"
  match:
    source: "All"
    scope: Namespaced
    kinds:
    - apiGroups: [ "*" ]
      kinds: [ "Pod" ]
```
