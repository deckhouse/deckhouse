{% raw %}

---
title: "Соберите образ контейнера"
permalink: en/modules-docs/chart-adapt/build-container-images/
---

Полезный подход - хранить образы для модулей в нашем registry. Очистите папку с образами `images/*` и загрузите туда наш образ для приложения **hello-world**.

```sh
rm -rf images/*
mkdir images/hello-world
echo "FROM quay.io/giantswarm/helloworld:0.2.0" > images/hello-world/Dockerfile
```

> Поддерживаются любые Docker файлы. Если необходимо собрать приложение из исходного кода, поместите его рядом с **Dockerfile** и включите его в образ с помощью команды `COPY`.

Чтобы использовать наш образ в шаблонах, замените его в манифестах на хелпер из библиотеки Deckhouse Kubernetes Platform.

```sh
sed -Ei '' 's/image\:(.*)/image: {{ include "helm_lib_module_image" (list . "helloWorld") }}/g' templates/deployment.yaml
```

Проверьте результат командой `cat` и убедитесь, что изменения применились.

> Можно пользоваться вспомогательными функциями из [библиотеки Deckhouse Kubernetes Platform](https://github.com/deckhouse/lib-helm/tree/main/charts/helm_lib).
{% endraw %}. 
