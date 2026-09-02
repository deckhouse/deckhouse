---
title: "Модуль multitenancy-manager: FAQ"
---

## Управление доступом к cluster-wide-ресурсам

### Что делать, если PersistentVolumeClaim отклонён с ошибкой «is not available to project»?

Указанный в `spec.storageClassName` StorageClass недоступен проекту. Вебхук отклоняет такой запрос с сообщением вида:

```text
[multitenancy] PersistentVolumeClaim "<OBJECT_NAME>" references "<RESOURCE_NAME>" which is not available to project "<PROJECT_NAME>". Ask the cluster administrator to grant it.
```

Чтобы просмотреть доступные StorageClass, выполните следующую команду:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

Если нужного StorageClass нет в списке, используйте доступный или попросите администратора кластера добавить его в [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy).

Аналогичным образом можно проверить доступность других cluster-wide-ресурсов (например, ClusterIssuer, ClusterRole, LoadBalancerClass) с помощью соответствующего [AvailableClusterResource](cr.html#availableclusterresource).

### Как узнать, какие политики доступа применяются к проекту?

Политика [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy) применяется к проекту, если значение `spec.projectSelector` в ней соответствует лейблам неймспейсов проекта.

Чтобы просмотреть все политики и их селекторы, выполните следующую команду:

```shell
d8 k get clusterresourcegrantpolicies -o yaml
```

Полученный список cluster-wide-ресурсов, доступных конкретному проекту, можно просмотреть с помощью [AvailableClusterResource](cr.html#availableclusterresource):

```shell
d8 k get available -n <PROJECT_NAME>
```

### Что произойдёт с существующими объектами после ограничения доступа?

Существующие объекты продолжат использовать ранее заданные значения. При обновлении объекта проверяются только новые значения, поэтому изменение политики доступа не нарушает работу уже созданных объектов.

Если существующий объект должен использовать другой cluster-wide-ресурс, явно измените соответствующее поле на доступное проекту значение.

Если существующий объект использует cluster-wide-ресурс, который больше не доступен проекту, срабатывает [алерт `ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation).

### Что означает алерт ClusterResourceGrantPolicyViolation?

Алерт означает, что один или несколько существующих объектов в проекте используют cluster-wide-ресурс, который больше не доступен проекту согласно текущим политикам.

Такие объекты продолжают работать. Чтобы устранить расхождение, предоставьте проекту доступ к используемому cluster-wide-ресурсу или измените объект так, чтобы он использовал доступный ресурс.

Подробную информацию о нарушениях можно посмотреть на дашборде Grafana в разделе «Security» → «Cluster Resource Grant Violations». Для мониторинга используется метрика `d8_cluster_objects_grant_violated`.

### Как разрешить все StorageClasses, кроме отдельных?

Чтобы запретить отдельные StorageClass, оставив остальные доступными, используйте параметр [`denied`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-denied) или [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector). Запрет имеет приоритет над разрешением.

Пример настройки приведён [в подразделе «Запрет отдельных ресурсов»](usage.html#запрет-отдельных-ресурсов).

### Что происходит, если ClusterResourceGrantPolicy отсутствует?

Если для ресурса не задана ни одна политика ClusterResourceGrantPolicy, его доступность определяется регистрацией: ресурс доступен всем проектам, если в GrantableClusterResourceDefinition задано [`defaultAvailability: All`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultavailability) (значение по умолчанию) и ресурс не попадает под фильтры [`excluded`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-excluded).

Например, определение `clusterroles`, поставляемое DKP, исключает все ClusterRole без лейбла `rbac.deckhouse.io/delegatable`, поэтому такие роли недоступны в RoleBinding даже при отсутствии политик.

Чтобы ограничить доступ, создайте [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy) и укажите проекты и доступные им cluster-wide-ресурсы.

### Я создал ClusterResourceGrantPolicy, но ничего не изменилось — почему?

Проверьте по порядку:

1. **Соответствие проекта селектору**. Убедитесь, что [`spec.projectSelector`](cr.html#clusterresourcegrantpolicy-v1alpha1-spec-projectselector) политики соответствует лейблам неймспейса проекта:

   ```shell
   d8 k get ns <PROJECT_NAME> --show-labels
   ```

1. **Доступные ресурсы проекта**. Проверьте, отражает ли AvailableClusterResource ожидаемые настройки политики:

   ```shell
   d8 k get available <RESOURCE_NAME> -n <PROJECT_NAME> -o yaml
   ```

1. **Регистрация cluster-wide-ресурса**. Убедитесь, что для значения `resourceName` существует соответствующий [ресурс GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition).

1. **Регистрация ссылки**. Если ожидается проверка определённого поля, убедитесь, что оно зарегистрировано с помощью [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference) и ссылка успешно связана с соответствующим GrantableClusterResourceDefinition.

Описание состояния регистрации приведено [в разделе «Проверка состояния регистрации ресурса»](usage.html#проверка-состояния-регистрации-ресурса).

### Как управление доступом к cluster-wide-ресурсам взаимодействует с RBAC?

Это независимые механизмы. RBAC определяет, *кто может выполнять* операции с объектом, а механизм управления доступом к cluster-wide-ресурсам — *какие cluster-wide-ресурсы* может использовать этот объект.

Например, для создания PersistentVolumeClaim пользователю необходимо иметь соответствующие права RBAC, а указанный в `.spec.storageClassName` StorageClass должен быть доступен проекту.

### Что произойдёт при отключении модуля multitenancy-manager?

После отключения модуля проверка доступности cluster-wide-ресурсов и автоматическая подстановка значений по умолчанию не выполняются. Существующие объекты при этом не изменяются.

Проверка полей с расширением `x-deckhouse-grantable-resource` в настройках приложений DKP также не выполняется.

Роль `d8:use:dict` в модуле `user-authz` продолжает работать.

### Что произойдёт при включении управления доступом на существующем кластере?

Существующие объекты продолжат использовать ранее заданные значения. Проверка доступа применяется к новым объектам, а при обновлении существующих объектов — только к новым значениям.

Поэтому перед созданием политик доступа изменять существующие объекты не требуется.

### Что произойдёт с объектами, созданными до ограничения доступа?

Такие объекты продолжат использовать ранее заданные значения. Новые объекты и новые значения в полях существующих объектов проверяются в соответствии с текущими политиками доступа.

Если существующий объект использует cluster-wide-ресурс, который больше не доступен проекту, срабатывает [алерт `ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation).
