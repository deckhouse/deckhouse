---
title: Управление проектами
permalink: ru/admin/multitenancy/project-management.html
description: Управление проектами
lang: ru
---

В Deckhouse Kubernetes Platform есть набор шаблонов для создания проектов:

- `default` — шаблон для базовых сценариев использования проектов:
  * ограничение ресурсов;
  * сетевая изоляция;
  * автоматические алерты и сбор логов;
  * выбор профиля безопасности;
  * настройка администраторов проекта.

- `secure` — включает все возможности шаблона `default`, а также дополнительные функции:
  * настройка допустимых для проекта UID/GID;
  * правила аудита обращения Linux-пользователей проекта к ядру;
  * сканирование запускаемых образов контейнеров на наличие известных уязвимостей (CVE).

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также дополнительные функции:
  * определение селектора узла для всех подов в проекте: если под создан, селектор узла пода будет автоматически **заменён** на селектор узла проекта;
  * определение стандартных tolerations для всех подов в проекте: если под создан, стандартные значения tolerations **добавляются** к нему автоматически.

Чтобы просмотреть все доступные параметры для шаблона проекта, выполните команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Создание проекта

1. Для создания проекта создайте кастомный ресурс [Project](/modules/multitenancy-manager/cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-projecttemplatename).
1. В параметре [.spec.parameters](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) кастомного ресурса [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate).

   Пример создания проекта с помощью [Project](/modules/multitenancy-manager/cr.html#project) из `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) представлен ниже:

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

### Автоматическое создание проекта для неймспейса

Для неймспейса возможно создать новый проект. Для этого пометьте неймспейс аннотацией `projects.deckhouse.io/adopt`. Например:

1. Создайте новый неймспейс:

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

   В списке проектов появится новый проект, соответствующий неймспейсу:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

Шаблон созданного проекта можно изменить на существующий.

{% alert level="warning" %}
Обратите внимание, что при смене шаблона может возникнуть конфликт ресурсов: если в чарте шаблона прописаны ресурсы, которые уже присутствуют в неймспейсе, то применить шаблон не получится.
{% endalert %}

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

## Использование лейблов для управления ресурсами

При создании ресурсов в ProjectTemplate можно использовать специальные лейблы для управления поведением `multitenancy-manager` при обработке этих ресурсов:

### Пропуск создания лейбла `heritage: multitenancy-manager`

По умолчанию все ресурсы, созданные из ProjectTemplate, получают лейбл `heritage: multitenancy-manager`.  
Он запрещают изменение ресурсов пользователями или любым контроллером, кроме `multitenancy-manager`.  
Если необходимо разрешить изменение ресурса (например, для совместимости с другими системами, или в случае реализации собственного контроля изменения создаваемых объектов), добавьте к ресурсу лейбл `projects.deckhouse.io/skip-heritage-label`.

Пример:

{% raw %}

```yaml
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

- Будут созданы **только один раз** при создании проекта.
- **Не будут обновляться** при последующих изменениях шаблона или обновлениях.
- Не будут отслеживаться в статусе проекта.
- Получат лейблы `projects.deckhouse.io/project` и `projects.deckhouse.io/project-template`, но **не получат** лейбл `heritage: multitenancy-manager`.

{% alert level="warning" %}
После того как ресурс помечен как `unmanaged`, он будет создан при первой установке, но не будет обновляться при изменении ProjectTemplate.
После создания ресурс становится полностью независимым и должен управляться вручную.
{% endalert %}

## Реализация валидации изменений объектов с помощью пользовательского лейбла

Модуль `multitenancy-manager` использует ValidatingAdmissionPolicy для защиты ресурсов с лейблом `heritage: multitenancy-manager` от ручных изменений.  
Вы можете реализовать аналогичную валидацию для ресурсов с любым лейблом.

### Принцип работы валидации в multitenancy-manager

`multitenancy-manager` валидирует объекты с лейблом `heritage: multitenancy-manager`.  
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
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Замените на ваш сервисный аккаунт.
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
   - `request.userInfo.username` — имя сервисного аккаунта, которому разрешено изменять ресурсы (замените на ваш сервисный аккаунт);
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

Модуль `multitenancy-manager` позволяет администраторам кластера управлять для каждого проекта, какие
кластерные ресурсы (напр. `StorageClass`, `ClusterIssuer`, `ClusterRole`, `LoadBalancerClass`) можно
использовать из неймспейсов проектов, и какое значение используется по умолчанию.

Это отдельный механизм от RBAC: RBAC решает, *кто может создать* объект, гранты — *какие кластерные
ресурсы этот объект может ссылаться*.

В механизме участвуют четыре кастомных ресурса:

- `GrantableClusterResourceDefinition` (`gcrd`, cluster-scoped) — регистрирует кластерный ресурс как
  управляемый грантами (поставляется платформой и модулями).
- `GrantableClusterResourceReference` (`gcrr`, cluster-scoped) — объявляет, какое поле какого CRD
  валидируется/подставляется (поставляется модулями).
- `ClusterResourceGrantPolicy` (`crgp`, cluster-scoped) — списки разрешений/запретов и дефолты
  администратора для проекта (единственный ручной шаг для контроля доступа).
- `AvailableClusterResource` (`available`, namespaced, read-only) — формируемый контроллером каталог,
  который проект читает, чтобы узнать доступные ресурсы.

Пока администратор не создал `ClusterResourceGrantPolicy`, все ресурсы доступны (разрешающий дефолт).
Валидация применяется только к неймспейсам проектов; существующие объекты при UPDATE сохраняют свои
значения (grandfathering).

Полное руководство — сценарии для администратора (ограничение StorageClasses, ClusterIssuers,
ClusterRoles, LoadBalancerClasses), обнаружение ресурсов тенантами, руководство для разработчиков
модулей, правила валидации и дефолтинга, мониторинг — см. в
[руководстве по использованию модуля multitenancy-manager](/products/kubernetes-platform/documentation/v1/modules/160-multitenancy-manager/usage_ru.html#управление-доступом-к-кластерным-ресурсам-гранты).
