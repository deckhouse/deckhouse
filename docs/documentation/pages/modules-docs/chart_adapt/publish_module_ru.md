---
title: "Опубликуйте модуль"
permalink: ru/modules-docs/chart-adapt/publish-module/
lang: ru
---

В `.gitlab-ci.yml` впишите свои переменные вместо указанных в шаблоне.

```yaml
MODULES_MODULE_NAME: echoserver
MODULES_REGISTRY: registry.flant.com
MODULES_MODULE_SOURCE: registry.flant.com/deckhouse/modules/template
MODULES_MODULE_TAG: ${CI_COMMIT_REF_NAME}
```

В Gitlab добавьте секреты для аутентификации в container registry в секции Settings -> CI/CD.

Например:

```text
MODULES_REGISTRY_LOGIN = username
MODULES_REGISTRY_PASSWORD = password
```

> **NOTE:** если вы используете fox, то доступы указывать не нужно.

Запушим наши изменения обратно в git.

```sh
rm -rf .tmp-chart
git add .
git commit -m "Initial Commit"
git push --set-upstream origin example
```
<!-- TODO: Сквош коммитов? -->

 Увидим, что сборка прошла успешна.

![Pipeline](../../../images/modules-docs/pipeline.png)

Теперь повесим тег v0.0.1. Во вновь появившемся окне нажимаем кнопку `Deploy to alpha`.

![Deploy](../../../images/modules-docs/deploy.png)

После этого модуль доступен для подключения в кластерах Deckhouse.
