---
title: Проекты
permalink: ru/test/admin/projects.html
lang: ru
---

В данном разделе описаны процессы работы **администратора [системы](../concepts/glossary.html#система)** с [проектами](../concepts/glossary.html#проект) в Deckhouse Kubernetes Platform (DKP). Процессы работы **администратора [проекта](glossary.html#проект)** описаны в разделе [Использование](../user/projects.html).

> РЕМАРКА: администратора **системы**. Да?  (админ проекта читает раздел использование)

## Шаблоны проектов

Шаблоны проектов в DKP описывают конфигурацию проекта, которая автоматически применяется ко всем создаваемым в рамках проекта объектам в кластере — [пространствам имён](../concepts/glossary.html#пространство-имён), квотам, ролям, политикам и др. DKP включает несколько [встроенных шаблонов проектов](#встроенные-шаблоны-проектов), которые охватывают типовые сценарии их применения. Также на основе встроенных шаблонов можно [создавать собственные](#создание-собственного-шаблона-проекта).

### Встроенные шаблоны проектов

{% alert level="info" %}
Встроенные шаблоны проектов недоступными для редактирования, но могут быть использованы для создания новых шаблонов проектов под конкретные задачи и требования.
{% endalert %}

В DKP доступны следующие шаблоны проектов:

- `default` — шаблон для базовых сценариев использования проектов. Особенности шаблона:
  - управление ограничением ресурсов;
  - настройка сетевой изоляции;
  - настройка оповещений и правил сбора логов;
  - установка профиля безопасности;
  - указание администраторов проекта.

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — включает все возможности шаблона `default`, а также дополнительные функции:
  - настройка допустимых для проекта UID/GID;
  - настройка правил аудита обращения Linux-пользователей проекта к ядру;
  - включение сканирования запускаемых образов контейнеров на наличие известных уязвимостей (CVE).

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — включает все возможности шаблона `secure`, а также дополнительные функции:
  - определение селектора узла для всех подов в проекте: если под создан, селектор узла пода будет автоматически **заменён** на селектор узла проекта;
  - определение стандартных tolerations для всех подов в проекте: если под создан, стандартные значения tolerations **добавляются** к нему автоматически.

  Описание шаблона [в GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

Чтобы перечислить все доступные параметры для шаблона проекта, выполните команду:

```shell
d8 k get projecttemplates <TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

### Создание собственного шаблона проекта

DKP позволяет создавать собственные [шаблоны проектов](#шаблоны-проектов) на основе [встроенных](#встроенные-шаблоны-проектов).

Для создания своего шаблона:

1. Возьмите за основу один из шаблонов по умолчанию, например, `default`.
1. Скопируйте его в отдельный файл, например, `my-project-template.yaml` при помощи команды:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Отредактируйте файл `my-project-template.yaml`, внесите в него необходимые изменения.

   > Необходимо изменить не только шаблон, но и схему входных параметров под него.
   >
   > Шаблоны для проектов поддерживают все [функции шаблонизации Helm](https://helm.sh/docs/chart_template_guide/function_list/).
1. Измените имя шаблона в поле `.metadata.name`.
1. Примените полученный шаблон командой:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Проверьте доступность нового шаблона с помощью команды:

   ```shell
   d8 k get projecttemplates <ИМЯ_НОВОГО_ШАБЛОНА>
   ```

## Создание проекта

Для создания [проекта](../concepts/glossary.html#проект) используются следующие ресурсы:

- [ProjectTemplate](#TODO) — описывает шаблон проекта. Задает список ресурсов и уровни доступа, которые будут созданы в рамках проекта. Также он содержит схему параметров, которые можно передать при создании проекта;
- [Project](#TODO) — описывает конкретный проект, связанный с шаблоном проекта.

При создании проекта из определенного ProjectTemplate происходит следующее:

1. Переданные [параметры](#TODO cr.html#project-v1alpha2-spec-parameters) валидируются по OpenAPI-спецификации (параметр [openAPI](#TODO cr.html#projecttemplate-v1alpha1-spec-parametersschema) ресурса [ProjectTemplate](#TODO cr.html#projecttemplate));
1. Выполняется рендеринг [шаблона для ресурсов](#TODO cr.html#projecttemplate-v1alpha1-spec-resourcestemplate) с помощью [Helm](https://helm.sh/docs/). Значения для рендеринга берутся из параметра [parameters](#TODO cr.html#project-v1alpha2-spec-parameters) ресурса [Project](#TODO cr.html#project);
1. Создаётся пространство имён с именем, которое совпадает c именем [Project](#TODO cr.html#project);
1. По очереди создаются все ресурсы, описанные в шаблоне.

{% alert level="warning" %}
При изменении шаблона проекта, все созданные проекты будут обновлены в соответствии с новым шаблоном.
{% endalert %}

### Web UI

Процесс создания проекта в веб-интерфейсе Deckhouse выглядит следующим образом:

1. Выберите вкладку Проекты и нажмите на кнопку "Создать проект".
1. В появившемся окне заполните следующие поля:
   - Сведения о проекте (Project);
   - Доступ на проект (ProjectRoleBinding);
   - Возможности (Features);
   - Шаблон проекта;
   - Настройки для пространства имён (ProjectTemplate);
   - Запросы и лимиты (ЦП, Память, Хранилище).
1. Нажмите на кнопку "Сохранить".

<!--
- Рабочий вариант, будет изменен позже.
    ![project-creation](../../images/multitenancy(test)/project-creation.png)
-->

### CLI

1. Для создания проекта создайте Custom Resource [Project](cr.html#project) с указанием имени шаблона проекта в поле [.spec.projectTemplateName](cr.html#project-v1alpha2-spec-projecttemplatename).
1. В параметре [.spec.parameters](cr.html#project-v1alpha2-spec-parameters) укажите значения параметров для секции [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) кастомного ресурса `ProjectTemplate`.

   Пример создания проекта с помощью [Project](cr.html#project) из `default` [ProjectTemplate](cr.html#projecttemplate) представлен ниже:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example project.
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

   Успешно созданный проект должен отображаться в статусе `Deployed` (синхронизирован). Если отображается статус `Error` (ошибка), добавьте аргумент `-o yaml` к команде (например, `kubectl get projects my-project -o yaml`) для получения более подробной информации о причине ошибки.

## Редактирование проекта

### Web UI

Процесс редактирования [проекта](../concepts/glossary.html#проект) в веб-интерфейсе Deckhouse выглядит следующим образом:

1. На вкладке "Проекты" нажмите на название нужного проекта, чтобы открыть меню редактирования.
1. На вкладке "Конфигурация" внесите изменения в необходимые поля:
   - Сведения о проекте (Project);
   - Доступ на проект (ProjectRoleBinding);
   - Возможности (Features);
   - Шаблон проекта;
   - Настройки для пространства имён (ProjectTemplate);
   - Запросы и лимиты (ЦП, Память, Хранилище).
1. При необходимости откройте вкладку Meta для добавления меток и аннотаций или вкладку YAML для ручного редактирования всего манифеста.
1. После всех изменений нажмите на кнопку "Сохранить".

<!--
- Рабочий вариант, будет изменен позже.
    ![project-editing](../../images/multitenancy(test)/project-editing.png)
-->

### CLI

Пример (#TODO):

## Удаление проекта

### Web UI

Процесс удаления [проекта](../concepts/glossary.html#проект) в веб-интерфейсе Deckhouse выглядит следующим образом:

1. На вкладке "Проекты" нажмите на название нужного проекта, чтобы открыть меню редактирования.
1. Нажмите на значок корзины в правом верхнем углу вкладки "Конфигурация" и подтвердите удаление проекта.

### CLI

Пример (#TODO):

## Совместимость с Kubernetes

DKP поддерживает совместимость с Kubernetes, расширяя механизм пространства имён при использовании [проектов](concepts.html#проект), и позволяет автоматически мигрировать существующие пространства имён в проекты.

### Автоматическое создание проекта для пространства имён

{% alert level="warning" %}
Механизм работает только если системный флаг `AllowNamespacesWithoutProjects` установлен в `true`.
{% endalert %}

DKP может автоматически создавать новые [проекты](../concepts/glossary.html#проект) из существующих [пространств имён](../concepts/glossary.html#пространство-имён). Для этого пометьте необходимое пространство имён аннотацией `projects.deckhouse.io/adopt`. Например:

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

   ```console
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

1. При необходимости, измените шаблон созданного проекта.

{% alert level="warning" %}
Обратите внимание, что при смене шаблона может возникнуть конфликт ресурсов: если в чарте шаблона прописаны ресурсы, которые уже присутствуют в пространстве имён, то применить шаблон не получится.
{% endalert %}

## Дополнительные ресурсы

- [Управление полезной нагрузкой](../user/projects.html) внутри проекта
- [Архитектура проектов](../architecture/projects.html)
