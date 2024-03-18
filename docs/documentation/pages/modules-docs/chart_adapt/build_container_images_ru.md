---
title: "Соберите образ контейнера"
permalink: ru/modules-docs/chart-adapt/build-container-images/
lang: ru
---

Полезный подход - хранить образы для модулей в нашем реестре (registry). Очистите папку с образами (`images/*`) и загрузите туда наш образ для приложения **hello-world**.

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
