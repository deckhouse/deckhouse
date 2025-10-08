---
title: "Проекты"
permalink: ru/stronghold/documentation/admin/platform-management/access-control/projects.html
lang: ru
---

## Описание

Проекты (ресурс [Project](/modules/multitenancy-manager/cr.html#project)) в платформе обеспечивают изолированные окружения для создания ресурсов пользователя.

Настройки проекта позволяют задать квоты для ресурсов, ограничить сетевое взаимодействие, как внутри платформы, так и вне её.

Для создания проектов используются шаблоны (ресурс [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate)).

{% alert level="warning" %}
При изменении шаблона проекта, все созданные проекты будут обновлены в соответствии с новым шаблоном.
{% endalert %}

## Шаблоны для проектов доступные по умолчанию

Платформа включает следующий набор шаблонов для создания проектов:

- `empty` — пустой шаблон без предопределенных ресурсов;

- `default` — шаблон для базовых сценариев использования проектов:
  - ограничение ресурсов;
  - сетевая изоляция;
  - автоматические алерты и сбор логов;
  - выбор профиля безопасности;
  - настройка для администрирования проекта.

- `secure` — включает все возможности шаблона `default`, а также дополнительные функции:
  - правила аудита обращения Linux-пользователей проекта к ядру.

Чтобы получить список всех доступных параметров для шаблона проекта, выполните следующую команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Создание проекта

1. Чтобы создать проект, создайте ресурс [Project](/modules/multitenancy-manager/cr.html#project), указав имя шаблона проекта в поле `.spec.projectTemplateName`.
1. В параметре `.spec.parameters` ресурса [Project](/modules/multitenancy-manager/cr.html#projecttemplate).

   Пример создания проекта с помощью ресурса Project из `default` ProjectTemplate представлен ниже:

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

1. Чтобы проверить статус проекта, выполните команду:

   ```shell
   d8 k get projects my-project
   ```

   Если проект успешно создан, его статус будет `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `d8 k get projects my-project -o yaml`), чтобы получить более подробную информацию о причине ошибки.

## Создание собственного шаблона для проекта

Шаблоны проектов по умолчанию включают базовые сценарии использования и служат примером возможностей шаблонов.

Для создания собственного шаблона проекта выполните следующие шаги:

1. Выберите один из шаблонов по умолчанию, например, `default`.

1. Скопируйте его в отдельный файл (например, `my-project-template.yaml`) с помощью команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Откройте файл `my-project-template.yaml` и отредактируйте его, чтобы внести необходимые изменения.

   > Важно изменить не только сам шаблон, но и схему параметров, соответствующую этому шаблону.
   >
   > Шаблоны проектов поддерживают все [функции шаблонизации Helm](https://helm.sh/docs/chart_template_guide/function_list/).

1. Измените имя шаблона в поле `.metadata.name`.

1. Примените измененный шаблон с помощью команды:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Чтобы проверить, доступен ли новый шаблон, выполните команду:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```
