---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

В Deckhouse Kubernetes Platform есть набор шаблонов для создания проектов:

- `default` — шаблон для базовых сценариев использования проектов:
  - ограничение ресурсов;
  - сетевая изоляция;
  - автоматические алерты и сбор логов;
  - выбор профиля безопасности;
  - настройка администраторов проекта.

    Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — включает все возможности шаблона `default`, а также дополнительные функции:
  - настройка допустимых для проекта UID/GID;
  - правила аудита обращения Linux-пользователей проекта к ядру;
  - сканирование запускаемых образов контейнеров на наличие известных уязвимостей (CVE).

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также дополнительные функции:
  - определение селектора узла для всех подов в проекте: если под создан, селектор узла пода будет автоматически **заменён** на селектор узла проекта;
  - определение стандартных tolerations для всех подов в проекте: если под создан, стандартные значения tolerations **добавляются** к нему автоматически.

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

Чтобы перечислить все доступные параметры для шаблона проекта, выполните команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Создание проекта

1. Для создания проекта создайте ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha2-spec-projecttemplatename).
1. В параметре [.spec.parameters](cr.html#project-v1alpha2-spec-parameters) ресурса Project укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) ресурса ProjectTemplate.

   Пример создания проекта с помощью ресурса [Project](cr.html#project) из `default` [ProjectTemplate](cr.html#projecttemplate) представлен ниже:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     parameters:
       resourceQuota:
         requests:
           cpu: 5
           memory: 5Gi
           storage: 1Gi
         limits:
           cpu: 5
           memory: 5Gi
       networkPolicy: Isolated
       podSecurityProfile: Restricted
       extendedMonitoringEnabled: true
       administrators:
       - subject: Group
         name: k8s-admins
   ```

1. Для проверки статуса проекта выполните команду:

   ```shell
   d8 k get projects my-project
   ```

   Успешно созданный проект должен отображаться в статусе `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `d8 k get projects my-project -o yaml`) для получения более подробной информации о причине ошибки.

### Автоматическое создание проекта для пространства имён

Для пространства имён возможно создать новый проект. Для этого пометьте пространство имён аннотацией `projects.deckhouse.io/adopt`. Например:

1. Создайте новое пространство имён:

   ```shell
   d8 k create ns test
   ```

1. Пометьте его аннотацией:

   ```shell
   d8 k annotate ns test projects.deckhouse.io/adopt=""
   ```

1. Убедитесь, что проект создался:

   ```shell
   d8 k get projects
   ```

   В списке проектов появится новый проект, соответствующий пространству имён:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

Шаблон созданного проекта можно изменить на существующий.

{% endraw %}

{% alert level="warning" %}
Обратите внимание, что при смене шаблона может возникнуть конфликт ресурсов: если в чарте шаблона прописаны ресурсы, которые уже присутствуют в пространстве имён, то применить шаблон не получится.
{% endalert %}

{% raw %}

## Создание собственного шаблона для проекта

Шаблоны проектов по умолчанию включают базовые сценарии использования и служат примером возможностей шаблонов.

Для создания своего шаблона:

1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
1. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Отредактируйте файл `my-project-template.yaml`, внесите в него необходимые изменения.

   {% alert level="info" %}
   Необходимо изменить не только шаблон, но и схему входных параметров под него.

   Шаблоны для проектов поддерживают все [функции шаблонизации Helm](https://helm.sh/docs/chart_template_guide/function_list/).
   {% endalert %}

1. Измените имя шаблона в поле `.metadata.name`.
1. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```

{% endraw %}

## Использование лейблов для управления ресурсами

При создании ресурсов в `ProjectTemplate` можно использовать специальные лейблы для управления поведением `multitenancy-manager` при обработке этих ресурсов:

### Пропуск создания лейбла `heritage: multitenancy-manager`

По умолчанию все ресурсы, созданные из `ProjectTemplate`, получают лейбл `heritage: multitenancy-manager`.  
Он запрещают изменение ресурсов пользователями или любым контроллером, кроме `multitenancy-manager`.  
Если необходимо разрешить изменение ресурса (например, для совместимости с другими системами, или в случае реализации собственного контроля изменения создаваемых объектов), добавьте к ресурсу лейбл `projects.deckhouse.io/skip-heritage-label`.

Пример:

{% raw %}

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/skip-heritage-label: "true"
    app: my-app
data:
  key: value
```

{% endraw %}

В этом случае ресурс получит лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но не получит лейбл `heritage: multitenancy-manager`.

### Исключение ресурсов из управления multitenancy-manager

Если необходимо исключить ресурс из управления `multitenancy-manager` (например, если он должен управляться вручную или другим контроллером), добавьте к ресурсу лейбл `projects.deckhouse.io/unmanaged`.

Пример:

{% raw %}

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: external-secret
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/unmanaged: "true"
type: Opaque
data:
  token: <base64-encoded-value>
```

{% endraw %}

Ресурсы с лейблом `projects.deckhouse.io/unmanaged`:

- Будут созданы **только один раз** при создании проекта;
- **Не будут обновляться** при последующих изменениях шаблона или обновлениях;
- Не будут отслеживаться в статусе проекта;
- Получат лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но **не получат** лейбл `heritage: multitenancy-manager`.

{% alert level="warning" %}
После того как ресурс помечен как `unmanaged`, он будет создан при первой установке, но не будет обновляться при изменении ProjectTemplate.
После создания ресурс становится полностью независимым и должен управляться вручную.
{% endalert %}

## Реализация валидации изменений объектов с помощью пользовательского лейбла

Модуль `multitenancy-manager` использует `ValidatingAdmissionPolicy` для защиты ресурсов с лейблом `heritage: multitenancy-manager` от ручных изменений.  
Вы можете реализовать аналогичную валидацию для ресурсов с любым лейблом.

### Как работает валидация в multitenancy-manager

Происходит валидация объектов с лейблом `heritage: multitenancy-manager`.  
Для этого используются следующие ресурсы:

1. ValidatingAdmissionPolicy — определяет правила валидации:
   - Операции: `UPDATE` и `DELETE`;
   - Проверка: разрешены только операции от имени service account контроллера;
   - Применяется ко всем ресурсам и API группам.

1. ValidatingAdmissionPolicyBinding — определяет на какие объекты распространяется валидация:
   - Использует `namespaceSelector` и `objectSelector` для выбора ресурсов по лейблу `heritage: multitenancy-manager`.

### Создание собственной валидации

Для реализации валидации для ресурсов с другим лейблом (например, `heritage: my-custom-label`):

1. Создайте файл с манифестами ресурсов ValidatingAdmissionPolicy и ValidatingAdmissionPolicyBinding:

   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicy
   metadata:
     name: my-custom-label-validation
   spec:
     failurePolicy: Fail
     matchConstraints:
       resourceRules:
         - apiGroups:   ["*"]
           apiVersions: ["*"]
           operations:  ["UPDATE", "DELETE"]
           resources:   ["*"]
           scope: "*"
     validations:
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Замените на ваш service account.
         reason: Forbidden
         messageExpression: 'object.kind == ''Namespace'' ? ''This resource is managed by '' + object.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + object.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-label-validation
   spec:
     policyName: my-custom-label-validation
     validationActions: [Deny, Audit]
     matchResources:
       namespaceSelector:
         matchLabels:
           heritage: my-custom-label
       objectSelector:
         matchLabels:
           heritage: my-custom-label
   ```

1. Настройте параметры валидации:

   - `policyName` — уникальное имя политики (должно совпадать с `Policy` и `Binding`);
   - `request.userInfo.username` — имя service account, которому разрешено изменять ресурсы (замените на ваш service account);
   - `heritage: my-custom-label` — значение лейбла `heritage` для ваших ресурсов (замените на ваше значение). Запрещено использование значение `multitenancy-manager`, `deckhouse`;
   - `failurePolicy: Fail` — политика при ошибке валидации:
     - `Fail` — отклонять запрос при ошибке проверки,
     - `Ignore` — игнорировать ошибки валидации.
   - `validationActions` — действия валидации:
     - `Deny` — отклонять неразрешенные операции,
     - `Audit` — записывать операции в аудит лог.

1. Примените политику:

   ```shell
   d8 k apply -f my-validation-policy.yaml
   ```

1. Убедитесь, что ваши ресурсы имеют соответствующий лейбл `heritage`:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: my-resource
     labels:
       heritage: my-custom-label
   ```

## Управление доступом к cluster-wide-ресурсам

Модуль позволяет управлять доступом проектов к cluster-wide-ресурсам,
таким как StorageClass, ClusterIssuer, ClusterRole и другим.

Описание механизма, используемых ресурсов и cluster-wide-ресурсов, зарегистрированных платформой,
приведено [на странице описания модуля](./#управление-доступом-к-cluster-wide-ресурсам).

Далее приведены основные сценарии настройки и использования механизма.

### Для администраторов кластера

Ниже приведены примеры настройки доступа проектов к cluster-wide-ресурсам с помощью ClusterResourceGrantPolicy.

#### Ограничение StorageClass для проекта

Чтобы разрешить проектам использовать только StorageClass с именами `fast-ssd` и `standard`, а `fast-ssd` использовать по умолчанию, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-storage
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: storageclasses
      default: fast-ssd
      allowed:
        - fast-ssd
        - standard
```

{% endraw %}

При создании PersistentVolumeClaim без значения `spec.storageClassName` в это поле автоматически подставляется `fast-ssd`. Если указан StorageClass, которого нет в списке разрешённых, создание PersistentVolumeClaim отклоняется.

Для StorageClass используется режим подстановки значения по умолчанию [`Coerce`](cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting). Если встроенный admission-контроллер Kubernetes уже подставил в `spec.storageClassName` класс по умолчанию, недоступный проекту, значение заменяется на `fast-ssd`, а создание PersistentVolumeClaim не отклоняется.

Чтобы проверить, какие StorageClass доступны проекту, выполните следующую команду:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

#### Ограничение ClusterIssuer для проекта

Чтобы разрешить проектам использовать только ClusterIssuer с именами `letsencrypt-prod` и `vault-issuer`, а `letsencrypt-prod` использовать по умолчанию, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-issuers
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: clusterissuers
      default: letsencrypt-prod
      allowed:
        - letsencrypt-prod
        - vault-issuer
```

{% endraw %}

Политика применяется к ClusterIssuer, указанному одним из следующих способов:

- в поле `.spec.issuerRef.name` ресурса Certificate, если в `.spec.issuerRef.kind` указано ClusterIssuer;
- в аннотации `cert-manager.io/cluster-issuer` ресурса Ingress.

При создании Certificate с ClusterIssuer без значения `.spec.issuerRef.name` в это поле автоматически подставляется `letsencrypt-prod`. Если указан ClusterIssuer, которого нет в списке разрешённых, создание Certificate отклоняется.

Для аннотации `cert-manager.io/cluster-issuer` значение по умолчанию не подставляется. Если аннотация указана, её значение проверяется по тому же списку разрешённых ClusterIssuer.

{% alert level="info" %}
Управление доступом к ClusterIssuer доступно только при включённом [модуле `cert-manager`](/modules/cert-manager/).
{% endalert %}

#### Предоставление доступа к дополнительным ClusterRole

По умолчанию в RoleBinding можно использовать только ClusterRole с лейблом `rbac.deckhouse.io/delegatable`.

Чтобы разрешить проектам команды `payments` использовать дополнительные ClusterRole, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: extra-roles
spec:
  projectSelector:
    matchLabels:
      team: payments
  resources:
    - resourceName: clusterroles
      allowed:
        - my-custom-role
      allowedSelector:
        matchLabels:
          shared: "true"
```

{% endraw %}

Политика дополнительно разрешает использовать:

- ClusterRole с именем `my-custom-role`;
- ClusterRole, соответствующие селектору `shared: "true"`.

ClusterRole с лейблом `rbac.deckhouse.io/delegatable` при этом остаются доступными.

При создании или изменении RoleBinding указанная в нём ClusterRole проверяется на доступность проекту. Значение ClusterRole автоматически не подставляется.

#### Ограничение LoadBalancerClass для сервиса

Чтобы разрешить проектам использовать только LoadBalancerClass со значениями `internal-lb` и `edge-lb`, а `internal-lb` использовать по умолчанию, создайте следующий [ресурс ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: lb-classes
spec:
  projectSelector:
    matchLabels:
      environment: staging
  resources:
    - resourceName: loadbalancerclasses
      default: internal-lb
      allowed:
        - internal-lb
        - edge-lb
```

{% endraw %}

В отличие от StorageClass, ClusterIssuer и ClusterRole, ресурс LoadBalancerClass не является отдельным ресурсом Kubernetes. Политика определяет допустимые значения поля `.spec.loadBalancerClass` для сервисов типа `LoadBalancer`.

При создании Service типа `LoadBalancer` без значения `.spec.loadBalancerClass` в это поле автоматически подставляется `internal-lb`. Если указано значение, которого нет в списке разрешённых, создание Service отклоняется.

На Service других типов политика не распространяется.

#### Предоставление доступа ко всем ресурсам определённого типа

Чтобы разрешить определённым проектам использовать все ресурсы выбранного типа без явного перечисления, установите [`availabilityDefault: All`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-availabilitydefault).

Следующая политика разрешает всем проектам с лейблом `environment: sandbox` использовать любые StorageClass:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: open-storage-for-sandbox
spec:
  projectSelector:
    matchLabels:
      environment: sandbox
  resources:
    - resourceName: storageclasses
      availabilityDefault: All
```

{% endraw %}

Обычно для управления доступом достаточно явно указывать разрешённые ресурсы с помощью [`allowed`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowed) или [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector). Используйте параметр `availabilityDefault: All`, если выбранным проектам необходимо предоставить доступ ко всем ресурсам указанного типа.

#### Запрет отдельных ресурсов

Чтобы запретить проектам использовать отдельные ресурсы, оставив остальные доступными, используйте параметр [`denied`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-denied) или [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector).

Следующая политика запрещает проектам с лейблом `environment: dev` использовать StorageClass с именами `expensive-nvme` и `archived-hdd`:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: deny-expensive-storage
spec:
  projectSelector:
    matchLabels:
      environment: dev
  resources:
    - resourceName: storageclasses
      denied:
        - expensive-nvme
        - archived-hdd
```

{% endraw %}

Запрет имеет приоритет над разрешением. Если ресурс соответствует одновременно `denied` или `deniedSelector` и `allowed` или `allowedSelector`, он считается недоступным.

#### Управление доступом с помощью label-селекторов

Чтобы управлять доступом к ресурсам без перечисления их имён, используйте параметры [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector) и [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector). Селекторы позволяют разрешать или запрещать ресурсы на основе их лейблов.

Следующая политика разрешает проектам с лейблом `tier: shared` использовать StorageClass с лейблом `shared: "true"`, за исключением StorageClass с лейблом `deprecated: "true"`:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: shared-storage-only
spec:
  projectSelector:
    matchLabels:
      tier: shared
  resources:
    - resourceName: storageclasses
      allowedSelector:
        matchLabels:
          shared: "true"
      deniedSelector:
        matchLabels:
          deprecated: "true"
```

{% endraw %}

### Для пользователей проекта

Пользователи проекта могут просматривать доступные им cluster-wide-ресурсы, а также проверять значения, используемые по умолчанию.

#### Просмотр доступных cluster-wide-ресурсов

В неймспейсе каждого проекта автоматически создаётся [ресурс AvailableClusterResource](cr.html#availableclusterresource) для каждого зарегистрированного
ресурса. С их помощью можно узнать, какие cluster-wide-ресурсы доступны проекту и какой ресурс используется по умолчанию.

Чтобы посмотреть все доступные cluster-wide-ресурсы проекта, выполните следующую команду:

```shell
d8 k get available -n <PROJECT_NAME>
```

Пример вывода:

```text
NAME                KIND         DEFAULT      AVAILABLE   AGE
storageclasses      StorageClass fast-ssd     2           5m
clusterissuers      ClusterIssuer letsencrypt 2           5m
```

Чтобы просмотреть подробную информацию о ресурсах определённого типа (например, StorageClass), выполните следующую команду:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

#### Отказ в использовании cluster-wide-ресурса

Если при создании или изменении объекта указанный cluster-wide-ресурс недоступен проекту, операция отклоняется с сообщением:

```text
resource <RESOURCE_NAME> is not available to project <PROJECT_NAME>
```

В этом случае проверьте доступные ресурсы с помощью AvailableClusterResource. Используйте ресурс из списка доступных или попросите администратора кластера добавить необходимый ресурс.

#### Автоматическая подстановка значений по умолчанию

Для некоторых кластерных ресурсов администратор может задать значение по умолчанию. Если при создании объекта соответствующее значение не указано, оно подставляется автоматически.

Например, если для StorageClass по умолчанию задан `fast-ssd`, при создании PersistentVolumeClaim без `.spec.storageClassName` в это поле может быть автоматически подставлено значение `fast-ssd`.

Значение можно указать явно, выбрав любой доступный проекту ресурс из соответствующего AvailableClusterResource.

### Для разработчиков модулей

#### Настройка проверки ссылки на cluster-wide-ресурс

Если ресурс модуля содержит поле со ссылкой на cluster-wide-ресурс, уже зарегистрированный с помощью [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition), создайте [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference). Он определяет, в каких ресурсах и полях используется cluster-wide-ресурс, а также позволяет настроить проверку доступности и автоматическую подстановку значения по умолчанию.

Например, чтобы настроить проверку StorageClass, указанного в поле `.spec.storageClassName` ресурса PostgresDatabase, добавьте в Helm-чарт модуля следующий ресурс:

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: postgresdatabases-storageclasses
  labels:
    heritage: deckhouse
    module: postgres
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: ["postgres.example.com"]
    apiVersions: ["*"]
    resources: ["postgresdatabases"]
  fieldPaths:
    - path: $.spec.storageClassName
      defaulting: Coerce
```

{% endraw %}

В этом примере `storageclasses` — имя существующего GrantableClusterResourceDefinition, а `Coerce` позволяет при создании PostgresDatabase подставить доступный проекту StorageClass по умолчанию, если значение отсутствует или недоступно проекту.

Описание параметров GrantableClusterResourceReference, режимов подстановки значений по умолчанию, условий `match` и настройки ресурсов с несколькими API-версиями приведено [в описании ресурса](cr.html#grantableclusterresourcereference).

#### Регистрация нового cluster-wide-ресурса

Чтобы добавить управление доступом к новому cluster-wide-ресурсу, выполните следующее:

1. Создайте ресурс [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition), чтобы зарегистрировать cluster-wide-ресурс в механизме управления доступом.
1. Создайте один или несколько ресурсов [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference), чтобы определить поля, в которых используются ссылки на него.

   Например, чтобы зарегистрировать ресурс MyClusterResource, добавьте в Helm-чарт модуля следующий ресурс:

   {% raw %}

   ```yaml
   apiVersion: multitenancy.deckhouse.io/v1alpha1
   kind: GrantableClusterResourceDefinition
   metadata:
     name: myclusterresources
     labels:
       heritage: deckhouse
       module: my-module
   spec:
     grantedResource:
       apiGroup: my.example.com
       kind: MyClusterResource
     enforcement: Managed
     defaultAvailability: All
     excluded:
       - matchExpressions:
           - key: my.example.com/internal
             operator: Exists
   ```

   {% endraw %}

   В этом примере зарегистрированные ресурсы по умолчанию доступны проектам. Ресурсы с меткой `my.example.com/internal` исключаются из доступных.

   Описание параметров ресурса GrantableClusterResourceDefinition и доступных режимов управления приведено [в описании ресурса](cr.html#grantableclusterresourcedefinition).

1. После регистрации cluster-wide-ресурса настройте ссылки на него с помощью GrantableClusterResourceReference, как описано [в подразделе «Настройка проверки ссылки на cluster-wide-ресурс»](#настройка-проверки-ссылки-на-cluster-wide-ресурс).

#### Использование x-deckhouse-grantable-resource в настройках приложений DKP

Для управления доступом к cluster-wide-ресурсам в настройках приложений DKP используйте OpenAPI-расширение `x-deckhouse-grantable-resource`. В этом случае deckhouse-контроллер автоматически проверяет доступность указанного ресурса и при необходимости подставляет значение по умолчанию. Создавать GrantableClusterResourceReference вручную не требуется.

Описание расширения и примеры использования приведены [в разделе «Разработка приложений»](/products/kubernetes-platform/documentation/v1/architecture/marketplace/application-development.html#подстановка-значения-из-грантов-на-ресурсы-кластера-x-deckhouse-grantable-resource).

#### Проверка состояния регистрации ресурса

Состояние GrantableClusterResourceDefinition и связанных с ним ресурсов GrantableClusterResourceReference можно проверить в поле `status`:

- [`GrantableClusterResourceDefinition.status.references`](cr.html#grantableclusterresourcedefinition-v1alpha1-status-references) — содержит список связанных ресурсов `GrantableClusterResourceReference` и информацию о ресурсах, к которым они применяются;
- [`GrantableClusterResourceReference.status.bound`](cr.html#grantableclusterresourcereference-v1alpha1-status-bound) — указывает, найден ли соответствующий GrantableClusterResourceDefinition;
- `GrantableClusterResourceReference.status.conditions[Bound]` — содержит состояние привязки: `Resolved`, если определение найдено, или `UnknownResource`, если оно отсутствует. Состояние `UnknownResource` может указывать на ошибку в имени GrantableClusterResourceDefinition или на отсутствие необходимой регистрации.
