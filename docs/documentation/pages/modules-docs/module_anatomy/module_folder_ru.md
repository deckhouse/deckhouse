---
title: "Папка с модулем"
permalink: ru/modules-docs/module-anatomy/module-folder/
lang: ru
---

Модуль представляет собой папку с определенным набором вложенных файлов и папок. Ближайший аналог -- Helm chart.

## Структура модуля

```tree
📁 my-module/
├─ 📁 charts/
│  └─ 📁 helm_lib/
├─ 📁 crds/
│  ├─ 📝 crd1.yaml
│  ├─ 📝 doc-ru-crd1.yaml
│  ├─ 📝 crd2.yaml
│  └─ 📝 doc-ru-crd2.yaml
├─ 📁 docs/
│  ├─ 📝 README.md
│  ├─ 📝 README.ru.md (или README_RU.md)
│  ├─ 📝 EXAMPLES.md
│  ├─ 📝 EXAMPLES.ru.md (или EXAMPLES_RU.md)
│  ├─ 📝 CONFIGURATION.md
│  ├─ 📝 CONFIGURATION.ru.md (или CONFIGURATION_RU.md)
│  ├─ 📝 CR.md
│  ├─ 📝 CR.ru.md (или CR_RU.md)
│  ├─ 📝 FAQ.md
│  ├─ 📝 FAQ.ru.md
│  ├─ 📝 ADVANCED_USAGE.md
│  └─ 📝 ADVANCED_USAGE.ru.md
├─ 📁 hooks/
│  ├─ 📝 hook1.py
│  └─ 📝 hook2.py
├─ 📁 images/
│  ├─ 📁 nginx
│  │  └─ 📝 Dockerfile
│  └─ 📁 backend
│     └─ 📝 werf.inc.yaml
├─ 📁 openapi/
│  ├─ 📝 config-values.yaml
│  ├─ 📝 doc-ru-config-values.yaml
│  └─ 📝 values.yaml
├─ 📁 templates/
│  ├─ 📝 a.yaml
│  └─ 📝 b.yaml
├─ 📝 .helmignore
└─ 📝 Chart.yaml
└─ 📝 module.yaml
```

### charts

В папке `charts` лежат вспомогательные чарты, которые используются при рендере темплейтов.

> У Deckhouse есть своя библиотека вспомогательных функций -- [lib-helm](https://github.com/deckhouse/lib-helm), которую мы кладем в каждый модуль. Описание функций можно найти [здесь](https://github.com/deckhouse/lib-helm/blob/main/charts/helm_lib/README.md).
> Библиотеку можно положить в модуль как helm subchart. Для этого нужно скачать tgz-архив с нужным релизом и переместить его в папку `charts` в корне модуля. Архивы можно найти [по этой ссылке](https://github.com/deckhouse/lib-helm/releases/).

### crds

В этой папке лежат [Custom Resource Definition](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/), которые используются компонентами модуля. Логика применения CRD в Deckhouse отличается от той, что предлагает Helm -- они обновляются каждый раз при старте модуля, если есть что обновлять.

Для того чтобы CRD из папки начали применяться, необходимо добавить в папку hooks специальный хук (подробнее о хуках будет ниже).

Для того чтобы CRD из папки `/crds/` начали показываться в документации на сайте или в модуле `documentation` в кластере, нужно:
- создать файл перевода с аналогичной структурой, с полями description с префиксом `doc-ru-` в названии (например `/crds/doc-ru-crd.yaml` для `/crds/crd.yaml`). В файле перевода оставить только поля description, с переводом;
- создать файлы `/docs/CR.md` и `/docs/CR.ru.md` (или `CR_RU.md`).

### docs

В этой папке располагается документация к модулю:
- `README.md` -- описание, для чего нужен модуль и какую проблему он решает, общие архитектурные концепции

    Метаданные [front matter](https://gohugo.io/content-management/front-matter/) (должны быть во всех языковых версиях файла):
  - `title` - **(крайне желательно)** Заголовок страницы описания модуля. Пример — "Deckhouse web Admin console". Он же используется в breadcrumbs, если нет `linkTitle`.
  - `menuTitle` - **(желательно)** Название модуля в меню слева на странице документации. Пример - "Deckhouse admin". Если отсутствует, то используется название папки (репозитория), например `deckhouse-admin`.
  - `linkTitle` - **(опционально)** Отдельный заголовок для breadcrumbs, если, например, `title` очень длинный. Если отсутствует, то используется `title`.
  - `description` - **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов). Не повторяет title. Служит продолжением названия и раскрывает его детальнее. Используется при генерации preview-ссылок, индексации поисковиками. Пример (не самый хороший) — "Модуль позволяет полностью управлять кластером Kubernetes через веб-интерфейс, имея только навыки работы мышью."
  - `d8Edition` - **(опционально)** ee/ce. Минимальная редакция в которой доступен модуль. По умолчанию - ce.
  - `moduleStatus` - **(опционально)** experimental/{в будущем возможно будет proprietary, deprecated, whatever}. Некий статус модуля. Для experimental - выводится плашка на страницах о нестабильности текущего кода, и плашка в меню.

- `EXAMPLES.md` -- примеры конфигурации модуля с описанием.
    Метаданные [front matter](https://gohugo.io/content-management/front-matter/):
  - `title` - **(крайне желательно)** Заголовок страницы. Пример — "Examples". Он же используется в breadcrumbs, если нет `linkTitle`.
  - `description` - **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов). Не повторяет title. Служит продолжением названия и раскрывает его детальнее. Используется при генерации preview-ссылок, индексации поисковиками. Пример (не самый хороший) — "Примеры хранения секретов в нейронной сети с автоматической подстановкой в мысли при общении."
  - `linkTitle` - **(опционально)** Отдельный заголовок для breadcrumbs, если, например, `title` очень длинный. Если отсутствует, то используется `title`.
- `FAQ.md` -- часто задаваемые вопросы, чаще всего, касающиеся эксплуатации модуля (А как мне сделать А или Б?)
    Метаданные [front matter](https://gohugo.io/content-management/front-matter/):
  - `title` - **(крайне желательно)** Заголовок страницы (см. пример выше).
  - `description` - **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов) (см. пример выше).
  - `linkTitle` - **(опционально)** Отдельный заголовок для breadcrumbs, если, например, `title` очень длинный. Если отсутствует, то используется `title`.
- `ADVANCED_USAGE.md` -- инструкция по отладке модуля
    Метаданные [front matter](https://gohugo.io/content-management/front-matter/):
  - `title` - **(крайне желательно)** Заголовок страницы (см. пример выше).
  - `description` - **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов) (см. пример выше).
  - `linkTitle` - **(опционально)** Отдельный заголовок для breadcrumbs, если, например, `title` очень длинный. Если отсутствует, то используется `title`.
- `CR.md` и `CR.ru.md` (или `CR_RU.md`) -- файл, для генерации ресурсов из папки /crds/. Скоро будет генерироваться автоматически, но пока надо добавлять вручную.
   <details>
   <summary>Пример</summary>

   ```yaml
   ---
   title: "Custom resources"
   ---
   ```

   </details>
- `CONFIGURATION.md` -- файл, для генерации ресурсов из файлов `/openapi/config-values.yaml` и `/openapi/doc-<LANG>-config-values.yaml`.  Скоро будет генерироваться автоматически, но пока надо добавлять вручную.
   <details>
   <summary>Пример файла</summary>

   ```yaml
   ---
   title: "Настройки модуля"  <-- "Module configuration" для англоязычного файла
   ---
   ```

   </details>

#### Ассеты

Картинки, pdf-файлы и т.п. нужно складывать в папке docs или подпапке (например `/docs/images/`).

Все ссылки должны быть относительными.

Для каждого языка нужно положить свой файл с соответствующим суффиксом. Например `image1.jpg` и `image1.ru.jpg`. Ссылаться на них нужно соответственно как `[image1](image1.jpg)` в англоязычном документе и `[image1](image1.ru.jpg)` в русскоязычном.  

### hooks

Подробнее с концептом хуков можно ознакомиться в [документации shell-operator](https://flant.github.io/shell-operator/HOOKS.html).

Хуки нужны, чтобы модуль мог динамически работать с Kubernetes API. Например, чтобы среагировать на наличие объектов в кластере.

Каждый хук -- это отдельный исполняемый файл, который:
- При запуске с флагом `--config` выводит конфигурацию хука в формате YAML.
- При обычном запуске выполняет само действие.

> Для модулей Deckhouse поддерживается написание хуков только на языке Python.
>
> Файлы хуков должно иметь права на выполнение. Необходимо добавить их командой `chmod +x <путь до файла с хуком>`

<details>
  <summary>Пример хука ensure_crds.py</summary>
  
  ```python  
  import os

  import yaml
  from deckhouse import hook

  # We expect structure with possible subdirectories like this:
  #
  #   my-module/
  #       crds/
  #           crd1.yaml
  #           crd2.yaml
  #           subdir/
  #               crd3.yaml
  #       hooks/
  #           ensure_crds.py # this file


  config = """
  configVersion: v1
  onStartup: 5
  """


  def main(ctx: hook.Context):
      for crd in iter_manifests(find_crds_root(__file__)):
          ctx.kubernetes.create_or_update(crd)

  def iter_manifests(root_path: str):
    if not os.path.exists(root_path):
        return

    for dirpath, dirnames, filenames in os.walk(top=root_path):
        for filename in filenames:
            if not filename.endswith(".yaml"):
                # Wee only seek manifests
                continue
            if filename.startswith("doc-"):
                # Skip dedicated doc yamls, common for Deckhouse internal modules
                continue

        crd_path = os.path.join(dirpath, filename)
        with open(crd_path, "r", encoding="utf-8") as f:
            for manifest in yaml.safe_load_all(f):
                if manifest is None:
                    continue
                yield manifest

    for dirname in dirnames:
        subroot = os.path.join(dirpath, dirname)
        for manifest in iter_manifests(subroot):
            yield manifest


  def find_crds_root(hookpath):
      hooks_root = os.path.dirname(hookpath)
      module_root = os.path.dirname(hooks_root)
      crds_root = os.path.join(module_root, "crds")
      return crds_root


  if __name__ == "__main__":
      hook.run(main, config=config)
  ```

</details>

### images

В этой папке лежат инструкции для сборки образов модуля. Есть два варианта описания образа контейнера:

1. Привычный всем [Dockerfile](https://docs.docker.com/engine/reference/builder/).
2. Файл `werf.inc.yaml`, который является полным аналогом [секции описания образа из werf.yaml](https://werf.io/documentation/v1.2/reference/werf_yaml.html#L33).

> Структура папок должна быть плоской. Внутри папки images первый уровень это папки для файлов для сборки образов, второй уровень -- контекст сборки. Структура папок не может быть вложенной.

Собранные образы имеют content-based тэги, которые можно использовать в шаблонах образа следующим способом (при условии, что у вас подключен [lib-helm](https://github.com/deckhouse/lib-helm)).

```yaml
image: {{ include "helm_lib_module_image" (list . "<имя образа>") }}
```

> <имя образа> совпадает с именем папки в `images/` для этого модуля, переведенным в camel case с маленькой буквы.
> Пример: `images/echo-server` -> `echoServer`

### openapi

Служит для валидации входных параметров модуля. Содержит два файла:

#### config-values.yaml

Необходим, чтобы валидировать параметры модуля, которые может настраивать пользователь через `ModuleConfig`.

Для того чтобы схема начала показываться в документации на сайте или в модуле `documentation` в кластере, нужно:
- создать файл `doc-ru-config-values.yaml` со структурой, аналогичной структуре файла `config-values.yaml`. В файле `doc-ru-config-values.yaml` оставить только поля description, с переводом.
- создать файлы `/docs/CONFIGURATION.md` и `/docs/CONFIGURATION.ru.md` (или `CONFIGURATION_RU.md`) - это включит показ данных из файлов `/openapi/config-values.yaml` и `/openapi/doc-ru-config-values.yaml`

Пример схемы с одним настраиваемым параметром `nodeSelector`:

<details>
  <summary>openapi/config-values.yaml</summary>

  ```yaml
  type: object
  properties:
    nodeSelector:
      type: object
      additionalProperties:
        type: string
      description: >
        The same as the Pods' `spec.nodeSelector` parameter in Kubernetes.

        If the parameter is omitted or `false`, `nodeSelector` will be determined
        [automatically](https://deckhouse.io/documentation/v1/#advanced-scheduling).
  ```

</details>

Пример файла для русскоязычного перевода схемы:

<details>
  <summary>openapi/doc-ru-config-values.yaml</summary>

  ```yaml
  properties:
    nodeSelector:
      description: >
        Описание на русском языке. Разметка Markdown.
  ```

</details>

#### values.yaml

Необходим для валидации сходных данных для рендера шаблонов (чтобы не валидировать данные используя вспомогательные функции Helm-чарта).
Ближайший аналог -- [schema-файлы](https://helm.sh/docs/topics/charts/#schema-files) из Helm.

В `values.yaml` можно автоматически добавит валидацию параметров из `config-values.yaml`. В этом случае, инимальный `values.yaml` выглядит так:

<details>
  <summary>openapi/values.yaml</summary>

  ```yaml
  x-extend:
    schema: config-values.yaml
  type: object
  properties:
    internal:
      type: object
      default: {}
  ```

</details>

### templates

В этой директории лежат [шаблоны Helm](https://helm.sh/docs/chart_template_guide/getting_started/). Работают по тем же правилам.

> Для доступа к настройкам модуля в шаблонах нужно использовать путь `.Values.<имяМодуля>` (имя модуля конвертируется в формат camelcase с маленькой буквы), а для глобальных настроек `.Values.global`.
>
> Для удобства шаблонизации, используйте [lib-helm](https://github.com/deckhouse/lib-helm) -- набор вспомогательных функций использующих работы с глобальными и модульными values.
>
> Доступы в registry из ресурса `ModuleSource` доступны по пути `.Values.<имяМодуля>.registry.dockercfg`.
>
> Чтобы использовать их для пула образов в ваших контроллерах, создайте секрет и укажите его в поле `"imagePullSecrets": [{"name":"registry-creds"}]`
>
> ```yaml
> apiVersion: v1
> kind: Secret
> metadata:
>   name: registry-creds
> type: kubernetes.io/dockerconfigjson
> data:
>   .dockerconfigjson: {{ .Values.<имяМодуля>.registry.dockercfg }}
> ```

### .helmignore

Все что есть в папке модуля, в итоге попадает в секрет с Helm-релизом. Не попадают только файлы, указаные в `.helmignore`.
В случае модулей Deckhouse такие папки как `crds`, `images`, `hooks`, `openapi`, должны всегда быть добавлены в исключения в файле `.helmignore`, чтобы избежать превышения максимального размера Helm-релиза (1 Mb).

### Chart.yaml

Полный аналог [`Chart.yaml`](https://helm.sh/docs/topics/charts/#the-chartyaml-file) из Helm. Должен содержать в себе минимум поля `name` и `version`.

Пример:

```yaml
name: echoserver
version: 0.0.1
dependencies:
- name: deckhouse_lib_helm
  version: 1.5.0
  repository: https://deckhouse.github.io/lib-helm
```

### module.yaml

С помощью данного файла можно настроить модуль. На текущий момент поддерживаются следующие опции:

---
**tags**: [string] - дополнительные тэги для модуля, которые преобразуются в labels модуля: `module.deckhouse.io/$tag=""`

**weight**: integer - вес модуля. Вес по-умолчанию: 900, можно задать собственный вес в диапазоне 900 - 999.

**description**: string - описание модуля

Например:

```yaml
tags: ["test", "myTag"]
weight: 960
description: "my awesome module"
```

Будет создан модуль (`deckhouse.io/v1alpha/Module`) с labels: `module.deckhouse.io/test=""` и `module.deckhouse.io/myTag=""`, *весом* `960` и *description*: `my awesome module`.

Таким образом можно управлять очередностью модулей, а также задавать дополнительную метаинформацию для них.
