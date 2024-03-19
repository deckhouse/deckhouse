---
title: "Адаптируйте шаблоны"
permalink: ru/modules-docs/chart-adapt/modify-templates/
lang: ru
---


Выбранное имя будет соответствовать имени модуля в Deckhouse Kubernetes Platform. В некоторых местах оно может быть записано в формате kebab case или camel case. В инструкции следует использовать то же самое имя, которое было выбрано.

Откройте `Chart.yaml` и в параметре `name` впишите `hello-world`.

```sh
sed -Ei '' 's/^name:(.*)/name: hello-world/g' Chart.yaml
```

## Подготовьте шаблоны

1. Клонируйте исходный код чарта для hello-world.

   ```sh
   git clone https://github.com/giantswarm/hello-world-app .tmp-chart
   ```

2. Скопируйте шаблоны.

   ```sh
   rm -rf templates/*
   cp -fR .tmp-chart/helm/hello-world/templates/ templates/
   ```

3. Замените в шаблонах путь `.Values` на `.Values.helloWorld`.

   > Это соглашение, используемое в настоящее время в addon-operator, для доступа к значениям модуля. В будущих версиях планируется возможность отказа от этой архитектурной особенности.

   ```sh
   sed -i '' -e 's/.Values/.Values.helloWorld/g' $(find templates/ -type f)
   ```

## Добавьте схему для настроек

Чтобы пользователь настраивал модуль, необходимо добавить Open API схему для возможных опций. Это запретит пользователю вводить неверные настройки.

> Команда Deckhouse Kubernetes Platform старается тщательно подходить к выбору параметров, которые могут настраивать пользователи. Мы стремимся помочь пользователям, предоставляя возможность настраивать только те параметры, которые важны для их работы.

В Helm-чарте приложения `hello-world` уже имеется JSON-схема. Преобразуйте ее.

```sh
yq -P .tmp-chart/helm/hello-world/values.schema.json > openapi/config-values.yaml
```

Если в вашем чарте нет схемы, необходимо написать ее самостоятельно. Посмотрите примеры схем в репозитории, который клонировали на первом шаге.
