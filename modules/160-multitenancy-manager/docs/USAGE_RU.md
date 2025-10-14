---
title: "Модуль multitenancy-manager: примеры использования"
---
{% raw %}

## Шаблоны для проектов доступные по умолчанию

В Deckhouse Platform Certified Security Edition есть набор шаблонов для создания проектов:
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

Чтобы перечислить все доступные параметры для шаблона проекта, выполните команду:

```shell
d8 k get projecttemplates <ИМЯ_ШАБЛОНА_ПРОЕКТА> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Создание проекта

1. Для создания проекта создайте ресурс [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha2-spec-projecttemplatename).
2. В параметре [.spec.parameters](cr.html#project-v1alpha2-spec-parameters) ресурса `Project` укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) ресурса `ProjectTemplate`.

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

3. Для проверки статуса проекта выполните команду:

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
2. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Отредактируйте файл `my-project-template.yaml`, внесите в него необходимые изменения.

   > Необходимо изменить не только шаблон, но и схему входных параметров под него.
   >
   > Шаблоны для проектов поддерживают все функции шаблонизации Helm.
4. Измените имя шаблона в поле `.metadata.name`.
5. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

6. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```

{% endraw %}
