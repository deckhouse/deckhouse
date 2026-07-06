---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

В Deckhouse Kubernetes Platform есть набор шаблонов для создания проектов:

- `simple` — минимальный шаблон, создающий только пространство имён проекта (с возможностью задать дополнительные лейблы и аннотации). Используйте его, когда нужно лишь изолированное пространство имён, управляемое как проект, а доступ и ограничения настраиваются через [стандартные поля](#стандартные-поля-проекта) и [привязки ролей проекта](#предоставление-доступа-внутри-проекта).

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/simple.yaml).

- `default` — шаблон для базовых сценариев использования проектов:
  - сетевая изоляция;
  - автоматические алерты и сбор логов;
  - выбор профиля безопасности.

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

1. Для создания проекта создайте ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha3-spec-projecttemplatename).
1. Задайте [стандартные поля](#стандартные-поля-проекта) — [.spec.administrators](cr.html#project-v1alpha3-spec-administrators) и [.spec.quota](cr.html#project-v1alpha3-spec-quota), — которые теперь управляются непосредственно ресурсом Project независимо от шаблона.
1. В параметре [.spec.parameters](cr.html#project-v1alpha3-spec-parameters) ресурса Project укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) ресурса ProjectTemplate.

   Пример создания проекта с помощью ресурса [Project](cr.html#project) из `default` [ProjectTemplate](cr.html#projecttemplate) представлен ниже:

   ```yaml
   apiVersion: deckhouse.io/v1alpha3
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     # Стандартные поля, управляемые самим ресурсом Project.
     administrators:
       - kind: Group
         name: k8s-admins
     quota:
       requests.cpu: "5"
       requests.memory: 5Gi
       requests.storage: 1Gi
       limits.cpu: "5"
       limits.memory: 5Gi
     # Параметры конкретного шаблона.
     parameters:
       networkPolicy: Isolated
       podSecurityProfile: Restricted
       extendedMonitoringEnabled: true
   ```

   {% endraw %}

   {% alert level="info" %}
   API ресурса Project обслуживается как `deckhouse.io/v1alpha3`. Старые манифесты `v1alpha1`/`v1alpha2` продолжают работать: webhook конвертации автоматически переносит `parameters.administrators` и `parameters.resourceQuota` в стандартные поля `.spec.administrators` и `.spec.quota`.
   {% endalert %}

   {% raw %}

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

## Стандартные поля проекта

Администраторы проекта и квоты ресурсов больше не являются параметрами шаблона — это поля верхнего уровня ресурса [Project](cr.html#project), работающие с любым шаблоном (включая `simple` и проекты без шаблона):

- `.spec.administrators` — список субъектов (`kind: User` или `kind: Group` и `name`), получающих административный доступ к проекту. Контроллер реализует этот доступ через автоматически создаваемый [ProjectRoleBinding](cr.html#projectrolebinding) в пространстве имён проекта.
- `.spec.quota` — набор жёстких лимитов [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) (например, `requests.cpu`, `limits.memory`). Контроллер поддерживает `ResourceQuota` в пространстве имён проекта и сообщает текущее потребление в `.status.usage`.

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-project
spec:
  projectTemplateName: simple
  administrators:
    - kind: Group
      name: k8s-admins
  quota:
    requests.cpu: "5"
    requests.memory: 5Gi
    limits.cpu: "10"
    limits.memory: 10Gi
```

{% alert level="warning" %}
Объекты `ResourceQuota` и `AuthorizationRule`, описанные внутри шаблонов проектов, больше не отрисовываются: такие ресурсы теперь управляются исключительно через `.spec.quota` и `.spec.administrators`. Существующие шаблоны, в которых они объявлены, продолжают работать, но эти объекты отфильтровываются при рендеринге.
{% endalert %}

## Предоставление доступа внутри проекта

Чтобы предоставить доступ к пространствам имён проекта помимо администраторов проекта, используйте привязки ролей, которые ссылаются на кластерные роли и автоматически разворачиваются в нужные пространства имён проектов:

- [ProjectRoleBinding](cr.html#projectrolebinding) (пространство имён, короткое имя `prb`) — предоставляет роль в рамках **одного** проекта. Должен создаваться в главном пространстве имён проекта (имя которого совпадает с именем проекта). Контроллер создаёт `RoleBinding` в каждом пространстве имён этого проекта.
- [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) (кластерный, короткое имя `cprb`) — предоставляет роль во **всех** невиртуальных проектах. Контроллер создаёт `RoleBinding` в каждом пространстве имён каждого проекта и сообщает количество затронутых проектов в `.status.boundProjects`.

`roleRef` должен ссылаться на `ClusterRole`, имя которого начинается с одного из разрешённых префиксов (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). Проверка на повышение привилегий (через `SubjectAccessReview`) гарантирует, что запрашивающий пользователь имеет право привязать роль.

```yaml
---
apiVersion: deckhouse.io/v1alpha3
kind: ProjectRoleBinding
metadata:
  name: viewers
  namespace: my-project
spec:
  subjects:
    - kind: User
      name: viewer@example.com
  roleRef:
    kind: ClusterRole
    name: d8:project:viewer
---
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: platform-viewers
spec:
  subjects:
    - kind: Group
      name: platform
  roleRef:
    kind: ClusterRole
    name: d8:project:viewer
```

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

## Выдача кластерных ресурсов проектам

`multitenancy-manager` позволяет администраторам кластера управлять тем, какие кластерные ресурсы (например, StorageClass) можно использовать из неймспейсов проектов.

Для этого используются кастомные ресурсы:

- `GrantableClusterResourceDefinition` (cluster-scoped) — регистрирует кластерный ресурс, который
  можно выдавать проектам: какой это ресурс (`grantedResource`), где проверяются ссылки на него
  (`usageReferences`), базовая доступность (`defaultAvailability`) и как определяется дефолт проекта
  (`defaultFrom`). Каждая ссылка отдельно включает подстановку дефолта через `default: true` —
  ставьте его только для поля, значение которого ресурсу всегда нужно (например, `storageClassName`
  у `PersistentVolumeClaim`). Для ссылки, отсутствие которой осмысленно (например, аннотация-
  переключатель функции), не ставьте: такая ссылка по-прежнему проверяется и учитывается в квоте,
  но никогда не заполняется.
- `ClusterResourceGrantPolicy` (cluster-scoped) — выбирает проекты (по меткам неймспейса через
  `projectSelector`) и для каждого ресурса (`resourceName`) задаёт разрешённые имена (`allowed`,
  `allowedSelector`) и `default`. Allow-лист ограничивает ресурс этим списком.
- `AvailableClusterResource` (namespaced, read-only, короткое имя `available`) — формируемый контроллером каталог доступных для проекта кластерных ресурсов. Пользователи проекта читают его, чтобы узнать
  доступные имена.
- `ClusterResourceGrant` (namespaced) — пул объектной квоты проекта (лимиты на количество объектов и
  на измеряемые величины, например запрошенный объём хранилища). В статусе объекта отображается текущее потребление.

{% raw %}

```yaml
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  grantedResource:
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
  enforcement: Managed
  defaultAvailability: All
  defaultFrom:
    annotationKey: storageclass.kubernetes.io/is-default-class
  usageReferences:
    - rule:
        apiGroups:
          - ""
        apiVersions:
          - v1
        resources:
          - persistentvolumeclaims
      fieldPath: $.spec.storageClassName
      default: true
---
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
      default: fast-ssd          # Перекрывает дефолт по аннотации.
      allowed:
        - fast-ssd
        - standard
      allowedSelector:           # Плюс любой StorageClass с меткой shared=true.
        matchLabels:
          shared: "true"
```

{% endraw %}

Особенности применения:

- Проверяющий (validating) вебхук запрещает создание/обновление объектов в подходящих проектах, если
  используемое значение не разрешено. Уже присутствующие в объекте значения при обновлении не блокируются — существующие объекты продолжают работать.
- Мутирующий (mutating) вебхук подставляет значение по умолчанию только при создании и только в
  ссылки, помеченные `default: true`. Ссылки без неё (например, аннотации-переключатели) никогда
  не заполняются.
- Grant без совпавших проектов (или проект без совпавших grant’ов) ничего не ограничивает.
