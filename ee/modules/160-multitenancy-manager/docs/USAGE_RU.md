---
title: "Модуль multitenancy-manager: примеры конфигурации"
---
{% raw %}

## Создание изолированного окружения

Выполните следующие шаги, для создания изолированного окружения в кластере Kubernetes:

1. Создайте двух [статичных пользователей](../150-user-authn/usage.html#пример-создания-статического-пользователя), которым требуется дать доступ до изолированного окружения.

   Сохраните следующее содержимое (описание ресурсов `User`) в файл `users.yaml`:

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

   Выполните следующую команду, для создания пользователей:

   ```shell
   kubectl create -f users.yaml
   ```

   Проверьте что пользователи успешно создались, выполнив следующую команду:

   ```shell
   kubectl get users.deckhouse.io
   ```

   Пример вывода:

   ```shell
   NAME    EMAIL           GROUPS       EXPIRE_AT
   admin   admin@cluster   ["admins"]
   user    user@cluster    ["users"]
   ```

1. Создайте шаблон окружения с помощью ресурса [ProjectType](cr.html#projecttype):

   - в [.spec.subjects](cr.html#projecttype-v1alpha1-spec-subjects) опишите [роли](../../modules/140-user-authz/cr.html#authorizationrule-v1alpha1-spec-accesslevel), которые нужно выдать пользователям/группам/`ServiceAccount`'ам;
   - в [.spec.resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) опишите шаблоны ресурсов, которые требуется создать при настройке изолированных окружений;
   - в [.spec.openAPI](cr.html#projecttype-v1alpha1-spec-openapi) опишите спецификацию OpenAPI для значений (`values`), которые используются в описанных шаблонах ([.spec.resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate));
   - в [.spec.namespaceMetadata](cr.html#projecttype-v1alpha1-spec-namespacemetadata) опишите лейблы и аннотации, которые необходимо проставить на `Namespace` при настройке окружения.

   В параметре [.spec.subjects](cr.html#projecttype-v1alpha1-spec-subjects) шаблона описаны [роли](../../modules/150-user-authn/cr.html#user), которые требуется выдать созданным выше пользователям для новых окружений. В параметре [.spec.resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate) шаблона описываются три ресурса: `NetworkPolicy` (ограничивает сетевую доступность Подов вне создаваемого `Namespace`, кроме `kube-dns`), `LimitRange` и `ResourceQuota`. В шаблоне реурсов используются параметры, описанные в [.spec.openAPI](cr.html#projecttype-v1alpha1-spec-openapi) (`requests.cpu`, `requests.memory`, `requests.storage`, `limits.cpu`, `limit.memory`).

   Сохраните следующее содержимое (описание ресурса `ProjectType`) в файл `project-type.yaml`:

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

   Выполните следующую команду, для создания шаблона окружения:

   ```shell
   kubectl create -f project-type.yaml
   ```

   Проверьте что шаблон окружения успешно создался, выполнив следующую команду:

   ```shell
   kubectl get projecttypes.deckhouse.io
   ```

   Пример вывода:

   ```text
   NAME                READY   MESSAGE
   test-project-type   true
   ```

1. Создайте окружение с помощью ресурса [Project](cr.html#project), указав в поле [.spec.projectTypeName](cr.html#project-v1alpha1-spec-projecttypename) имя созданного ранее шаблона окружения. Поле [.spec.template](cr.html#project-v1alpha1-spec-template) заполните значениями, которые подходят для [.spec.openAPI ProjectType](cr.html#projecttype-v1alpha1-spec-openapi).

   Сохраните следующее содержимое (описание ресурса `Project`) в файл `project.yaml`:

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

   Выполните следующую команду, для создания окружения:

   ```shell
   kubectl create -f project.yaml
   ```

   Проверьте что окружение успешно создалось, выполнив следующую команду:

   ```shell
   kubectl get projects.deckhouse.io
   ```

   Пример вывода:

   ```shell
   NAME           READY   DESCRIPTION                              MESSAGE
   test-project   true    Test case from Deckhouse documentation
   ```

1. Проверьте ресурсы, созданные в рамках изолированного окружения.

   Пример команд и результата их работы:

   ```shell
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

1. [Сгенерируйте kubeconfig](../150-user-authn/faq.html#как-я-могу-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api) для доступа созданных пользователей к API-серверу.

1. Проверьте наличие доступа у созданных пользователей с помощью сгенерированного kubeconfig.

   Пример команд и результата их работы:

   ```shell
   $ kubectl get limitranges -n test-project --kubeconfig admin-kubeconfig.yaml
   NAME                          CREATED AT
   test-project-all-containers   2023-06-01T14:37:42Z
   
   $ kubectl get limitranges -n test-project --kubeconfig user-kubeconfig.yaml
   NAME                          CREATED AT
   test-project-all-containers   2023-06-01T14:37:42Z
   ```

{% endraw %}
