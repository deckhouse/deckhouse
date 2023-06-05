---
title: "Модуль multitenancy-manager: примеры конфигурации"
---

## Создание изолированного окружения

Чтобы создать изолированное окружение внутри kubernetes кластера требуется выполнить следующие шаги:

- Создайте двух [статичных пользователей](../../modules/150-user-authn/usage.html#пример-создания-статического-пользователя), которым требуется дать доступ до изолированного окружения (если пользователи уже есть - можно пропустить этот пункт):

```yaml
# users.yaml
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: user
spec:
  email: user@cluster
  # passwordUser
  password: $2a$10$yROPLTTMTI.AkkAskKGiUuQW3asoGosGgppj1NYXUboHx/onpGE7q
  groups:
    - users
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@cluster
  # passwordAdmin
  password: $2a$10$UpCxQCpMqJoVm53BvUyPluprS/mUtJ/yUoSuM8i3Z0TlbiBxGiB1q
  groups:
    - admins
```

```bash
$ kubectl create -f users.yaml
user.deckhouse.io/user created
user.deckhouse.io/admin created

$ kubectl get users.deckhouse.io
NAME    EMAIL           GROUPS       EXPIRE_AT
admin   admin@cluster   ["admins"]
user    user@cluster    ["users"]
```

- Создайте [ProjectType](cr.htlm#projecttype), который является шаблоном для создания [Project](cr.htlm#project) (изолированного окружения):

  - в [.spec.subjects](cr.htlm#projecttype-v1alpha1-spec-subjects) опишите [роли](../../modules/140-user-authz/cr.html#authorizationrule-v1alpha1-spec-accesslevel), которые нужно выдать пользователям/группам/`ServiceAccount`'ам;
  - в [.spec.resourcesTemplate](cr.htlm#projecttype-v1alpha1-spec-resourcestemplate) опишите шаблоны ресурсов, которые требуется создать при старте изолированных окружений;
  - в [.spec.openAPI](cr.htlm#projecttype-v1alpha1-spec-openapi) опишите спецификацию OpenAPI для значений (`values`) шаблонов в [.spec.resourcesTemplate](cr.htlm#projecttype-v1alpha1-spec-resourcestemplate);
  - в [.spec.namespaceMetadata](cr.htlm#projecttype-v1alpha1-spec-namespacemetadata) опишите лейблы и аннотации, которые необходимо проставить на `Namespace` при старте окружения.

  Например, в данном `ProjectType` в [.spec.subjects](cr.htlm#projecttype-v1alpha1-spec-subjects) описаны [роли](../../modules/150-user-authn/cr.html#user), которые требуется выдать созданным выше пользователям для новых окружений, а в [.spec.resourcesTemplate](cr.htlm#projecttype-v1alpha1-spec-resourcestemplate) описываются три ресурса: `NetworkPolivy`, который ограничивает сетевую доступность подов вне создаваемого `NS` (кроме `kube-dns`), `LimitRange` и `ResourceQuota`, в которых используются параметры из [.spec.openAPI](cr.htlm#projecttype-v1alpha1-spec-openapi) (`requests.cpu`, `requests.memory`, `requests.storage`, `limits.cpu`, `limit.memory`):

```yaml
# project-type.yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: ProjectType
metadata:
  name: test-project-type
spec:
  subjects:
    - kind: Group
      name: admins
      role: Admin
    - kind: Group
      name: users
      role: User
  namespaceMetadata:
    annotations:
      extended-monitoring.deckhouse.io/enabled: ""
    labels:
      created-from-project-type: test-project-type
  openAPI:
    requests:
      type: object
      properties:
        cpu:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: "^[0-9]+m?$"
        memory:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
        storage:
          type: string
          pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
    limits:
      type: object
      properties:
        cpu:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: "^[0-9]+m?$"
        memory:
          oneOf:
            - type: number
              format: int
            - type: string
          pattern: '^[0-9]+(\.[0-9]+)?(E|P|T|G|M|k|Ei|Pi|Ti|Gi|Mi|Ki)?$'
  resourcesTemplate: |
    ---
    # Max requests and limits for resource and storage consumption for all pods in a namespace.
    # Refer to https://kubernetes.io/docs/concepts/policy/resource-quotas/
    apiVersion: v1
    kind: ResourceQuota
    metadata:
      name: all-pods
    spec:
      hard:
        {{ with .params.requests.cpu }}requests.cpu: {{ . }}{{ end }}
        {{ with .params.requests.memory }}requests.memory: {{ . }}{{ end }}
        {{ with .params.requests.storage }}requests.storage: {{ . }}{{ end }}
        {{ with .params.limits.cpu }}limits.cpu: {{ . }}{{ end }}
        {{ with .params.limits.memory }}limits.memory: {{ . }}{{ end }}
    ---
    # Max requests and limits for resource consumption per pod in namespace.
    # All containers in a namespace must have requests and limits.
    # Refer to https://kubernetes.io/docs/concepts/policy/limit-range/
    apiVersion: v1
    kind: LimitRange
    metadata:
      name: all-containers
    spec:
      limits:
        - max:
            {{ with .params.limits.cpu }}cpu: {{ . }}{{ end }}
            {{ with .params.limits.memory }}memory: {{ . }}{{ end }}
          maxLimitRequestRatio:
            cpu: 1
            memory: 1
          type: Container
    ---
    # Deny all network traffic by default except namespaced traffic and dns.
    # Refer to https://kubernetes.io/docs/concepts/services-networking/network-policies/
    kind: NetworkPolicy
    apiVersion: networking.k8s.io/v1
    metadata:
      name: deny-all-except-current-namespace
    spec:
      podSelector:
        matchLabels: {}
      policyTypes:
        - Ingress
        - Egress
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: "{{ .projectName }}"
      egress:
        - to:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: "{{ .projectName }}"
        - to:
            - namespaceSelector:
                matchLabels:
                  kubernetes.io/metadata.name: kube-system
          ports:
            - protocol: UDP
              port: 53
```

```bash
$ kubectl create -f project-type.yaml
projecttype.deckhouse.io/test-project-type created

$ kubectl get projecttypes.deckhouse.io
NAME                READY   MESSAGE
test-project-type   true
```

- Создайте [Project](cr.htlm#project), у которого в поле [.spec.projectTypeName](cr.htlm#project-v1alpha1-spec-projecttypename) будет имя созданного ранее [ProjectType](cr.htlm#projecttype). Поле [.spec.template](cr.htlm#project-v1alpha1-spec-template) заполните значениями, которые подходят для [.spec.openAPI ProjectType](cr.htlm#projecttype-v1alpha1-spec-openapi):

```yaml
# project.yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: Project
metadata:
  name: test-project
spec:
  description: Test case from Deckhouse documentation
  projectTypeName: test-project-type
  template:
    requests:
      cpu: 5
      memory: 5Gi
      storage: 1Gi
    limits:
      cpu: 5
      memory: 5Gi
```

```bash
$ kubectl create -f project.yaml
project.deckhouse.io/test-project created

$ kubectl get projects.deckhouse.io
NAME           READY   DESCRIPTION                              MESSAGE
test-project   true    Test case from Deckhouse documentation
```

- Проверьте созданные ресурсы:

```bash
$ kubectl get -n test-project namespaces test-project
NAME           STATUS   AGE
test-project   Active   5m

$ kubectl get authorizationrules.deckhouse.io -n test-project
NAME                            AGE
test-project-admin-user-admin   5m
test-project-user-user-user     5m

$ kubectl get -n test-project resourcequotas
NAME                    AGE   REQUEST                                                              LIMIT
test-project-all-pods   5m   requests.cpu: 0/5, requests.memory: 0/5Gi, requests.storage: 0/1Gi   limits.cpu: 0/5, limits.memory: 0/5Gi

$ kubectl get -n test-project limitranges
NAME                          CREATED AT
test-project-all-containers   2023-06-01T14:37:42Z

$ kubectl get -n test-project networkpolicies.networking.k8s.io
NAME                                             POD-SELECTOR   AGE
test-project-deny-all-except-current-namespace   <none>         5m
```

- Сгенерируйте kubernetes конфиг для созданных пользователей с помощью модуля [user-authn](../../modules/150-user-authn/faq.html#как-я-могу-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api).

- Проверьте его работоспособность:

```bash
$ kubectl get limitranges -n test-project --kubeconfig admin-kubeconfig.yaml
NAME                          CREATED AT
test-project-all-containers   2023-06-01T14:37:42Z

$ kubectl get limitranges -n test-project --kubeconfig user-kubeconfig.yaml
NAME                          CREATED AT
test-project-all-containers   2023-06-01T14:37:42Z
```
