---
title: "Структура модуля"
permalink: ru/module-development/structure/
lang: ru
---

{% raw %}
Исходный код модуля и правила его сборки должны находиться в директории с определенной структурой. Ближайший аналог — Helm chart. На этой странице вы найдете описание директорий и файлов структуры модуля.

Мы подготовили репозиторий [шаблона модуля](https://github.com/deckhouse/modules-template/), содержащий структуру файлов и директорий, с которой мы рекомендуем начинать разработку модуля.

Пример структуры папки модуля, созданного из _шаблона_, содержащий правила сборки и публикации с помощью GitHub Actions:  

```tree
📁 my-module/
├─ 📁 .github/
│  ├─ 📁 workflows/
│  │  ├─ 📝 build_dev.yaml
│  │  ├─ 📝 build_prod.yaml
│  │  ├─ 📝 checks.yaml
│  │  ├─ 📝 deploy_dev.yaml
│  │  └─ 📝 deploy_prod.yaml
├─ 📁 .werf/
│  ├─ 📁 workflows/
│  │  ├─ 📝 bundle.yaml
│  │  ├─ 📝 images.yaml
│  │  ├─ 📝 images-digest.yaml
│  │  ├─ 📝 python-deps.yaml
│  │  └─ 📝 release.yaml
├─ 📁 charts/
│  └─ 📁 helm_lib/
├─ 📁 crds/
│  ├─ 📝 crd1.yaml
│  ├─ 📝 doc-ru-crd1.yaml
│  ├─ 📝 crd2.yaml
│  └─ 📝 doc-ru-crd2.yaml
├─ 📁 docs/
│  ├─ 📝 README.md
│  ├─ 📝 README.ru.md
│  ├─ 📝 EXAMPLES.md
│  ├─ 📝 EXAMPLES.ru.md
│  ├─ 📝 CONFIGURATION.md
│  ├─ 📝 CONFIGURATION.ru.md
│  ├─ 📝 CR.md
│  ├─ 📝 CR.ru.md
│  ├─ 📝 FAQ.md
│  ├─ 📝 FAQ.ru.md
│  ├─ 📝 ADVANCED_USAGE.md
│  └─ 📝 ADVANCED_USAGE.ru.md
├─ 📁 hooks/
│  ├─ 📝 ensure_crds.py
│  ├─ 📝 hook1.py
│  └─ 📝 hook2.py
├─ 📁 images/
│  ├─ 📁 nginx
│  │  └─ 📝 Dockerfile
│  └─ 📁 backend
│     └─ 📝 werf.inc.yaml
├─ 📁 lib/
│  └─ 📁 python/
│     └─ 📝 requirements.txt
├─ 📁 openapi/
│  ├─ 📁 conversions
│  │  ├─ 📁 testdata
│  │  │  ├─ 📝 v1-1.yaml
│  │  │  └─ 📝 v2-1.yaml
│  │  ├─ 📝 conversions_test.go
│  │  └─ 📝 v2.yaml
│  ├─ 📝 config-values.yaml
│  ├─ 📝 doc-ru-config-values.yaml
│  └─ 📝 values.yaml
├─ 📁 templates/
│  ├─ 📝 a.yaml
│  └─ 📝 b.yaml
├─ 📝 .helmignore
├─ 📝 Chart.yaml
├─ 📝 module.yaml
├─ 📝 werf.yaml
└─ 📝 werf-giterminism.yaml
```

## charts

В папке `/charts` находятся вспомогательные чарты Helm, которые используются при рендере шаблонов.

У Deckhouse Kubernetes Platform (DKP) существует собственная библиотека для работы с шаблонами – [lib-helm](https://github.com/deckhouse/lib-helm). О возможностях библиотеки можно почитать [в репозитории lib-helm](https://github.com/deckhouse/lib-helm/blob/main/charts/helm_lib/README.md). Чтобы положить библиотеку в модуль, загрузите [tgz-архив](https://github.com/deckhouse/lib-helm/releases/) с нужным релизом и переместите его в директорию `/charts` модуля.

## crds

В этой директории лежат [_CustomResourceDefinition_](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRD), которые используются компонентами модуля. CRD обновляются каждый раз, когда запускается модуль, если есть обновления.
{% endraw %}

{% alert level="warning" %}
Для того чтобы CRD из директории `/crds` модуля начали применяться в кластере, нужно добавить хук [hello.py](https://github.com/deckhouse/modules-template/blob/main/hooks/hello.py) из _шаблона модуля_. Подробнее о хуках в разделе [`hooks`](#hooks).
{% endalert %}

{% raw -%}
Чтобы отобразить CRD из директории `/crds` в документации на сайте или модуле documentation в кластере, выполните следующие шаги:
* создайте файл перевода со структурой аналогичной исходному файлу ресурса:
  - оставьте только параметры `description`, в которых укажите текст перевода;
  - используйте префикс `doc-ru-` в названии: например `/crds/doc-ru-crd.yaml` для `/crds/crd.yaml`.
* создайте файлы `/docs/CR.md` и `/docs/CR.ru.md`.

## docs

В папке `/docs` находится документация к модулю:

* `README.md` — описание, для чего нужен модуль, какую проблему он решает и общие архитектурные принципы.

  Метаданные файла ([front matter](https://gohugo.io/content-management/front-matter/)) в виде YAML-структуры должны быть во всех языковых версиях файла. Параметры, доступные для использования в метаданных:
  - `title` — **(рекомендуется)** Заголовок страницы описания модуля. Пример — "Веб-консоль администратора Deckhouse". Он же используется в навигации, если не указан параметр `linkTitle`.
  - `menuTitle` — **(желательно)** Название модуля в меню слева на странице (sidebar). Пример — "Deckhouse Admin". Если отсутствует, то используется название директории или репозитория, например `deckhouse-admin`.
  - `linkTitle` — **(опционально)** Отдельный заголовок для навигации, если, например, `title` очень длинный. Если отсутствует, то используется параметр `title`.
  - `description` — **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов). Не повторяет `title`. Служит продолжением названия и раскрывает его детальнее. Используется при генерации превью-ссылок и индексации поисковыми системами. Пример — «Модуль позволяет полностью управлять кластером Kubernetes через веб-интерфейс, имея только навыки работы мышью.»
  - `d8Edition` — **(опционально)** `ce/be/se/ee`. Минимальная редакция в которой доступен модуль. По умолчанию  — `ce`.
  - `moduleStatus` — **(опционально)** `experimental`. Статус модуля. Если модуль помечен как `experimental`, то на его страницах отображается предупреждение о том, что код нестабилен, а также отображается специальная плашка в меню.  

  <div markdown="0">
  <details><summary>Пример метаданных</summary>
  <pre class="highlight">
  <code>---
  title: "Веб-консоль администратора Deckhouse"
  menuTitle: "Deckhouse Admin"
  description: "Модуль позволяет полностью управлять кластером Kubernetes через веб-интерфейс, имея только навыки работы мышью."
  ---</code>
  </pre>
  </details>
  </div>

* `EXAMPLES.md` – примеры конфигурации модуля с описанием.
  
  Метаданные файла ([front matter](https://gohugo.io/content-management/front-matter/)) в виде YAML-структуры должны быть во всех языковых версиях файла. Параметры, доступные для использования в метаданных:
  - `title` – **(рекомендуется)** Заголовок страницы. Пример: "Примеры". Он же используется в навигации, если нет `linkTitle`.
  - `description` – **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов). Не повторяет `title`. Служит продолжением названия и раскрывает его детальнее. Используется при генерации превью-ссылок, индексации поисковиками. Пример: "Примеры хранения секретов в нейронной сети с автоматической подстановкой в мысли при общении."
  - `linkTitle` – **(опционально)** Отдельный заголовок для навигации, если, например, `title` очень длинный. Если отсутствует, то используется `title`.  

  <div markdown="0">
  <details><summary>Пример метаданных</summary>
  <pre class="highlight">
  <code>---
  title: "Примеры"
  description: "Примеры хранения секретов в нейронной сети с автоматической подстановкой в мысли при общении."
  ---</code>
  </pre>
  </details>
  </div>

* `FAQ.md` – часто задаваемые вопросы, касающиеся эксплуатации модуля ("Какой сценарий выбрать: А или Б?").
  
  Метаданные файла ([front matter](https://gohugo.io/content-management/front-matter/)) в виде YAML-структуры должны быть во всех языковых версиях файла. Параметры, доступные для использования в метаданных:
  - `title` – **(рекомендуется)** Заголовок страницы.
  - `description` – **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов).
  - `linkTitle` – **(опционально)** Отдельный заголовок для навигации, если, например, `title` очень длинный. Если отсутствует, то используется `title`.  

  <div markdown="0">
  <details><summary>Пример метаданных</summary>
  <pre class="highlight">
  <code>---
  title: "Часто задаваемые вопросы"
  description: "Часто задаваемые вопросы и ответы на них."
  ---</code>
  </pre>
  </details>
  </div>
  
* `ADVANCED_USAGE.md` -- инструкция по отладке модуля.
  
  Метаданные файла ([front matter](https://gohugo.io/content-management/front-matter/)) в виде YAML-структуры должны быть во всех языковых версиях файла. Параметры, доступные для использования в метаданных:
  - `title` – **(рекомендуется)** Заголовок страницы.
  - `description` – **(желательно)** Краткое уникальное описание содержимого страницы (до 150 символов).
  - `linkTitle` – **(опционально)** Отдельный заголовок для навигации, если, например, `title` очень длинный. Если отсутствует, то используется `title`.  

  <div markdown="0">
  <details><summary>Пример метаданных</summary>
  <pre class="highlight">
  <code>---
  title: "Отладка модуля"
  description: "В разделе разбираются все шаги по отладке модуля."
  ---</code>
  </pre>
  </details>
  </div>
  
* `CR.md` и `CR.ru.md` – файл для генерации ресурсов из папки `/crds/` добавьте вручную.  

  <div markdown="0">
  <details><summary>Пример метаданных</summary>
  <pre class="highlight">
  <code>---
  title: "Кастомные ресурсы"
  ---</code>
  </pre>
  </details>
  </div>

* `CONFIGURATION.md` – файл для создания ресурсов из `/openapi/config-values.yaml` и `/openapi/doc-<LANG>-config-values.yaml` добавьте вручную.  

  <div markdown="0">
  <details><summary>Пример метаданных</summary>
  <pre class="highlight">
  <code>---
  title: "Настройки модуля"
  ---</code>
  </pre>
  </details>
  </div>
  
Все изображения, PDF-файлы и другие медиафайлы нужно хранить в директории `/docs` или ее подкаталогах (например, `/docs/images/`). Все ссылки на файлы должны быть относительными.

Для каждого языка нужен файл с соответствующим суффиксом. Например, `image1.jpg` и `image1.ru.jpg`. Используйте ссылки:
- `[image1](image1.jpg)` в англоязычном документе;
- `[image1](image1.ru.jpg)` в русскоязычном документе.

## hooks

В директории `/hooks` находятся хуки модуля. Хук — это исполняемый файл, выполняемый при реакции на событие. Хуки используются модулем также для динамического взаимодействия с API Kubernetes. Например, они могут быть использованы для обработки событий, связанных с созданием или удалением объектов в кластере.
{% endraw %}

[Познакомьтесь](../#прежде-чем-начать) с концепцией хуков, прежде чем начать разрабатывать свой собственный хук. Для ускорения разработки хуков можно воспользоваться [Python-библиотекой](https://github.com/deckhouse/lib-python) от команды Deckhouse.

{% raw %}
Требования к работе хука:
- Хук должен быть написан на языке Python.
- При запуске с параметром `--config`, хук должен выводить свою конфигурацию в формате YAML.
- При запуске без параметров, хук должен выполнять свое основное действие.

Файлы хуков должны иметь права на выполнение. Добавьте их командой `chmod +x <путь до файла с хуком>`.

В репозитории [шаблона модуля](https://github.com/deckhouse/modules-template/) можно найти примеры хуков.

Пример хука, включающего в работу CRD (из директории [/crds](#crds) модуля):

```python
import os

import yaml
from deckhouse import hook

# Ожидается структура с возможными поддиректориями, подобная следующей:
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
                # Ищем только манифесты.
                continue
            if filename.startswith("doc-"):
                # Пропускаем YAML-файлы с документацией модуля.
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

## images

В директории `/images` находятся инструкции по сборке образов контейнеров модуля. На первом уровне находятся директории для файлов, используемых при создании образа контейнера, на втором — контекст для сборки.

Существует два способа описания образа контейнера:

1. [Dockerfile](https://docs.docker.com/engine/reference/builder/) — файл, который содержит команды для быстрой сборки образов. Если необходимо собрать приложение из исходного кода, поместите его рядом с Dockerfile и включите его в образ с помощью команды `COPY`.
2. Файл `werf.inc.yaml`, который является аналогом [секции описания образа из `werf.yaml`](https://werf.io/documentation/v1.2/reference/werf_yaml.html#L33).

Имя образа совпадает с именем директории для этого модуля, записанным в нотации _camelCase_ с маленькой буквы. Например, директории `/images/echo-server` соответствует имя образа `echoServer`.

Собранные образы имеют content-based теги, которые можно использовать в сборке других образов. Чтобы использовать content-based теги образов, [подключите библиотеку lib-helm](#charts). Вы также можете воспользоваться другими функциями [библиотеки helm_lib](https://github.com/deckhouse/lib-helm/tree/main/charts/helm_lib) Deckhouse Kubernetes Platform.

Пример использования content-based тега образа в Helm-чарте:

```yaml
image: {{ include "helm_lib_module_image" (list . "<имя образа>") }}
```

## openapi

### conversions

В директории `/openapi/conversions` находятся файлы конверсий параметров модуля и их тесты.

Конверсии параметров модуля позволяют конвертировать OpenAPI-спецификацию параметров модуля одной версии в другую. Конверсии могут быть необходимы в случаях, когда в новой версии OpenAPI-спецификации параметр переименовывается или переносится в другое место.

Каждая конверсия возможна только между двумя смежными версиями (например с первой версии на вторую). Конверсий может быть несколько, и цепочка конверсий должна покрывать все версии спецификации параметров, без "пропусков".

Файл конверсии, это YAML-файл произвольного имени следующего формата:

```yaml
version: N # Номер версии, в которую нужно выполнить конверсию. 
conversions: []  # Набор выражений jq, для преобразования данных из предыдущей версии.
```

Пример файла конверсии параметров модуля, когда в версии 2 удаляется параметр `.auth.password`:

```yaml
version: 2
conversions:
  - del(.auth.password) | if .auth == {} then del(.auth) end
```

#### Тесты конверсий

Для написания тестов конверсий можно использовать функцию `conversion.TestConvert`, которой нужно передать:
- путь до исходного файла конфигурации (версия до конвертации);
- путь до ожидаемого файла конфигурации (версия после конвертации).

[Пример](https://github.com/deckhouse/deckhouse/blob/main/modules/300-prometheus/openapi/conversions/conversions_test.go) теста конверсии.

## templates

В директории `/templates` находятся [шаблоны Helm](https://helm.sh/docs/chart_template_guide/getting_started/).

* Для доступа к настройкам модуля в шаблонах используйте путь `.Values.<имяМодуля>`, а для глобальных настроек `.Values.global`. Имя модуля конвертируется в нотации _camelCase_.

* Для упрощения работы с шаблонами используйте [lib-helm](https://github.com/deckhouse/lib-helm) – это набор дополнительных функций, которые облегчают работу с глобальными и модульными значениями.

* Доступы в registry из ресурса _ModuleSource_ доступны по пути `.Values.<имяМодуля>.registry.dockercfg`.

* Чтобы использовать эти функции для пула образов в контроллерах, создайте секрет и добавьте его в соответствующий параметр: `"imagePullSecrets": [{"name":"registry-creds"}]`.

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: registry-creds
  type: kubernetes.io/dockerconfigjson
  data:
    .dockerconfigjson: {{ .Values.<имяМодуля>.registry.dockercfg }}
  ```

Модуль может иметь параметры, с помощью которых может менять свое поведение. Параметры модуля и схема их валидации описываются в OpenAPI-схемах в директории `/openapi`.

Настройки лежат в двух файлах: [`config-values.yaml`](#config-valuesyaml) и [`values.yaml`](#valuesyaml).

Пример OpenAPI-схемы можно найти в [шаблоне модуля](https://github.com/deckhouse/modules-template/blob/main/openapi/config-values.yaml).

### config-values.yaml

Необходим для проверки параметров модуля, которые пользователь может настроить через [_ModuleConfig_](../../cr.html#moduleconfig).

Чтобы схема была представлена в документации на сайте или в модуле documentation в кластере, создайте:
- файл `doc-ru-config-values.yaml` со структурой, аналогичной структуре файла `config-values.yaml`. В файле `doc-ru-config-values.yaml` оставьте только переведенные параметры description;
- файлы `/docs/CONFIGURATION.md` и `/docs/CONFIGURATION.ru.md` — это включит показ данных из файлов `/openapi/config-values.yaml` и `/openapi/doc-ru-config-values.yaml`.

Пример схемы `/openapi/config-values.yaml` с одним настраиваемым параметром `nodeSelector`:

```yaml
type: object
properties:
  nodeSelector:
    type: object
    additionalProperties:
      type: string
    description: |
      The same as the Pods' `spec.nodeSelector` parameter in Kubernetes.

      If the parameter is omitted or `false`, `nodeSelector` will be determined
      [automatically](https://deckhouse.io/products/kubernetes-platform/documentation/v1/#advanced-scheduling).</code>
```

Пример файла `/openapi/doc-ru-config-values.yaml` для русскоязычного перевода схемы:

```yaml
properties:
  nodeSelector:
    description: |
      Описание на русском языке. Разметка Markdown.</code>
```

### values.yaml

Необходим для проверки исходных данных при рендере шаблонов без использования дополнительных функций Helm chart.
Ближайший аналог — [schema-файлы](https://helm.sh/docs/topics/charts/#schema-files) из Helm.

В `values.yaml` можно автоматически добавить валидацию параметров из `config-values.yaml`. В этом случае, минимальный `values.yaml` выглядит следующим образом:

```yaml
x-extend:
  schema: config-values.yaml
type: object
properties:
  internal:
    type: object
    default: {}
```

## .helmignore

Исключите файлы из Helm-релиза с помощью `.helmignore`. В случае модулей DKP директории `/crds`, `/images`, `/hooks`, `/openapi` обязательно добавляйте в `.helmignore`, чтобы избежать превышения лимита размера Helm-релиза в 1 Мб.

## Chart.yaml

Обязательный файл для чарта, аналогичный [`Chart.yaml`](https://helm.sh/docs/topics/charts/#the-chartyaml-file) из Helm. Должен содержать, как минимум, параметр `name` с именем модуля и параметр `version` с версией.

Пример:

```yaml
name: echoserver
version: 0.0.1
dependencies:
- name: deckhouse_lib_helm
  version: 1.38.0
  repository: https://deckhouse.github.io/lib-helm
```

## module.yaml

В данном файле настройте следующие опции модуля:
- `name: string` - имя модуля, например `echo-server`. Обязателен, при существовании данного файла.
- `tags: string` — дополнительные теги для модуля, которые преобразуются в лейблы модуля: `module.deckhouse.io/$tag=""`.
- `weight: integer` — вес модуля. Вес по-умолчанию: 900.
- `stage: string` — [cтадия жизненного цикла модуля](../versioning/#стадия-жизненного-цикла-модуля). Может быть `Sandbox`, `Incubating`, `Graduated` или `Deprecated`.
- `description: string` — описание модуля.
- `requirements: object` — зависимости модуля.
  - `deckhouse: string` — зависимость от версии Deckhouse Kubernetes Platform.
  - `kubernetes: string` — зависимость от версии Kubernetes.
  - `bootstrapped: boolean` — зависимость от стадии установки Deckhouse Kubernetes Platform.
- `disable: object` — опции отключения модуля.
  - `confirmation: boolean` — требовать подтверждение при отключении модуля.
  - `message: string` — сообщение с детализацией, что произойдет при отключении модуля.

Например:

```yaml
tags: ["test", "myTag"]
weight: 960
stage: "Sandbox"
description: "my awesome module"
requirements:
    deckhouse: ">= 1.61"
    kubernetes: ">= 1.27"
    bootstrapped: true
disable:
  confirmation: true
  message: "Disabling this module will delete all XXX resources."
```

Будет создан модуль (`deckhouse.io/v1alpha/Module`) с лейблами: `module.deckhouse.io/test=""` и `module.deckhouse.io/myTag=""`, весом `960` и описанием `my awesome module`.

Таким образом можно управлять очередностью модулей, а также задавать дополнительную метаинформацию для них.

Пример настройки зависимости от версии Deckhouse Kubernetes Platform:

```yaml
name: test
weight: 901
requirements:
    deckhouse: ">= 1.61"
```

Пример настройки зависимости от версии Kubernetes:

```yaml
name: test
weight: 901
requirements:
    kubernetes: ">= 1.27"
```

Пример настройки зависимости от статуса установки кластера (bootstrapped):

```yaml
name: ingress-nginx
weight: 402
description: |
    Ingress controller for nginx
    https://kubernetes.github.io/ingress-nginx

requirements:
    bootstrapped: true
```

{% endraw %}
