---
title: "Модуль documentation"
description: "Веб-интерфейс с документацией Deckhouse Kubernetes Platform."
webIfaces:
- name: documentation
---

Модуль `documentation` создает веб-интерфейс с документацией, соответствующей запущенной версии Deckhouse Kubernetes Platform.

Это может быть полезно, когда Deckhouse работает в сети с ограничением доступа в интернет.

Для получения адреса веб-интерфейса в шаблоне [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` замените на `documentation`.

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс документации будет доступен по адресу `documentation-kube.company.my`.
