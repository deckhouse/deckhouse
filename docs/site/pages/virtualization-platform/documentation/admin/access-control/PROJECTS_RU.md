---
title: "Проекты"
permalink: ru/virtualization-platform/documentation/admin/access-control/projects.html
lang: ru
---

## Описание

Проекты (ресурс [Project](../../../reference/cr.html#Project)) в платформе обеспечивают изолированные окружения для создания ресурсов пользователя.

Настройки проекта позволяют задать квоты для ресурсов, ограничить сетевое взаимодействие как внутри платформы так и с внешним миром.

Для создания проектов используются шаблоны (ресурс [ProjectTemplate](../../../reference/cr.html#ProjectTemplate)).

> **Внимание!** При изменении шаблона проекта, все созданные проекты будут обновлены в соответствии с новым шаблоном.

## Шаблоны для проектов доступные по умолчанию

Платформа включает следующий набор шаблонов для создания проектов:
- `empty` — пустой шаблон без предопределенных ресурсов;

- `default` — шаблон для базовых сценариев использования проектов:
  * ограничение ресурсов;
  * сетевая изоляция;
  * автоматические алерты и сбор логов;
  * выбор профиля безопасности;
  * настройка администраторов проекта.

- `secure` — включает все возможности шаблона `default`, а также дополнительные функции:
  * правила аудита обращения Linux-пользователей проекта к ядру;

Чтобы перечислить все доступные параметры для шаблона проекта, выполните команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Создание проекта

1. Для создания проекта создайте ресурс [Project](../../../reference/cr.html#Project) с указанием имени шаблона проекта в поле `.spec.projectTemplateName`.
2. В параметре `.spec.parameters` ресурса [Project](../../../reference/cr.html#Project) укажите значения параметров для секции `.spec.parametersSchema.openAPIV3Schema` ресурса [ProjectTemplate](../../../reference/cr.html#ProjectTemplate).

   Пример создания проекта с помощью ресурса [Project](../../../reference/cr.html#Project) из `default` [ProjectTemplate](../../../reference/cr.html#ProjectTemplate) представлен ниже:

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

3. Для проверки статуса проекта выполните команду:

   ```shell
   d8 k get projects my-project
   ```

   Успешно созданный проект должен отображаться в статусе `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `d8 k get projects my-project -o yaml`) для получения более подробной информации о причине ошибки.

## Создание собственного шаблона для проекта

Шаблоны проектов по умолчанию включают базовые сценарии использования и служат примером возможностей шаблонов.

Для создания своего шаблона:
1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
2. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Отредактируйте файл `my-project-template.yaml`, внесите в него необходимые изменения.

   > Необходимо изменить не только шаблон, но и схему входных параметров под него.
   >
   > Шаблоны для проектов поддерживают все [функции шаблонизации Helm](https://helm.sh/docs/chart_template_guide/function_list/).
4. Измените имя шаблона в поле `.metadata.name`.
5. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

6. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```
