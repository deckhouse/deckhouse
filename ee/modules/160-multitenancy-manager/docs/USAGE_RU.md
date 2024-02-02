---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

Deckhouse Kubernetes Platform включает следующий набор шаблонов для создания проектов:
- `default` — шаблон для базовых сценариев использования проектов:
  * ограничение ресурсов;
  * сетевая изоляция;
  * автоматические алерты и сбор логов;
  * выбор профиля безопасности;
  * настройка администраторов проекта.

- `secure` — включает в себя все возможности шаблона `default` и дополнительные возможности:
  * настройку допустимых для проекта UID/GID;
  * правила аудита обращения к ядру линукс пользоваетелей проекта;
  * сканирование запускаемых образов контейнеров на наличие CVE.

## Создание проекта

1. Для создания проекта создайте ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha1-spec-projecttemplate).
2. В параметре [.spec.template](cr.html#project-v1alpha1-spec-template) ресурса `Project` укажите значения параметров для секции [.spec.schema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec--schema-openAPIV3Schema) ресурса `ProjectTemplate`.

   Пример создания проекта с помощью ресурса [Project](cr.html#project) из `default` [ProjectTemplate](cr.html#projecttemplate) представлен ниже:

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

3. Для проверки статуса проекта выполните команду:

   ```shell
   kubectl get projects my-project
   ```

   Успешно созданный проект должен отображаться в статусе `Sync` (синхронизирован).

## Создание своего шаблона для проекта

Шаблоны проектов по умолчанию включают базовые сценарии использования и служат примером возможностей шаблонов.

Для создания своего шаблона:
1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
2. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   kubectl get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Отредактируйте файл `my-project-template.yaml`, внесите в него необходимые изменения.
   > Необходимо изменить не только шаблон, но и схему входных параметров под него.
4. Измените имя шаблона в поле [.metadata.name](cr.html#projecttemplate-v1alpha1-metadata-name).
5. Примените полученный шаблон командой:

    ```shell
    kubectl apply -f my-project-template.yaml
    ```

   > Шаблоны для проектов поддерживают все [функции шаблонизации Helm](https://helm.sh/docs/chart_template_guide/function_list/).

{% endraw %}
