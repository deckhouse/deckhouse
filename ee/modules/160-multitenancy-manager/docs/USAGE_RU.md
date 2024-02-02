---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

В поставку Deckhouse Kubernetes Platform включены следующие шаблоны для проектов:
- `default` — шаблон, который покрывает базовые сценарии использования проектов:
  * ограничение ресурсов;
  * сетевая изоляция;
  * автоматические алерты и сбор логов;
  * выбор профиля безопасности;
  * настройка администраторов проекта.

- `secure` — содержит в себе все возможности шаблона `default` и дополнительно:
  * настройка допустимых для проекта UID/GID;
  * правила аудита обращения к ядру линукс пользоваетелей проекта;
  * сканирование запускаемых образов контейнеров на наличие CVE.

## Создание проекта

Для создания проекта необходимо создать ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha1-spec-projecttemplate).
В поле [.spec.template](cr.html#project-v1alpha1-spec-template) ресурса `Project` необходимо указать значения параметров, которые подходят для [.spec.schema.openAPIV3Schema ProjectTemplate](cr.html#projecttemplate-v1alpha1-spec--schema-openAPIV3Schema).

Пример создания проекта с помощью ресурса [Project](cr.html#project) из `default` [ProjectTemplate](cr.html#projecttemplate):

```yaml
apiVersion: deckhouse.io/v1alpha1
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
    podSecurityPolicy: Restricted
    enableExtendedMonitoring: true
```

Чтобы посмотреть статус проекта, выполните команду:

```shell
kubectl get projects my-project
```

Успешно созданный проект должен быть в статусе "Синзронизирован".

## Создание своего шаблона для проекта

Шаблоны по умолчанию содержат в себе базовые сценарии использования проектов и служат хорошим примером применения возможностей шаблонизации.

Для создания своего шаблона:
1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
2. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   kubectl get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Отредактируйте файл `my-project-template.yaml`, внесите в него необходимые изменения. **Не забудьте** что изменить нужно не только шаблон, но и схему входных параметров под него.
4. Измените имя шаблона в поле [.metadata.name](cr.html#projecttemplate-v1alpha1-metadata-name).
5. Примените свой новый шаблон командой:

    ```shell
    kubectl apply -f my-project-template.yaml
    ```

> Шаблоны для проектов поддерживают все [функции шаблонизации Helm](https://helm.sh/docs/chart_template_guide/function_list/).

{% endraw %}
