---
title: "Соберите образ контейнера"
permalink: en/modules-docs/chart-adapt/build-container-images/
---

Одна из хороших практик - образы для модулей должны лежать в нашем registry. Очистим папку images/* и добавим туда наш образ для hello-world-app.

```sh
rm -rf images/*
mkdir images/hello-world
echo "FROM quay.io/giantswarm/helloworld:0.2.0" > images/hello-world/Dockerfile
```

> **NOTE:** Поддерживаются любые Docker-файлы. Если вам необходимо собрать приложение из исходников, положите их рядом с Dockerfile и добавьте при их в образ при помощи дериктивы COPY.

Теперь для того чтобы наш образ использовался в шаблонах, заменим его в манифестах на хелпер из библиотеки Deckhouse.

```sh
sed -Ei '' 's/image\:(.*)/image: {{ include "helm_lib_module_image" (list . "helloWorld") }}/g' templates/deployment.yaml
```

Можно проверить результат при помощи команды `cat` и убедиться, что изменения применились.

> **NOTE:** Можно также использовать другие вспомогательные функции из библиотеки Deckhouse. Подробнее смотрите в документации <https://github.com/deckhouse/lib-helm/tree/main/charts/helm_lib>
