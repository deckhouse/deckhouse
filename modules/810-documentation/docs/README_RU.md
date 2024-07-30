---
title: "Модуль documentation"
webIfaces:
- name: documentation
---

Этот модуль создает веб-интерфейс с документацией, соответствующей запущенной версии Deckhouse.

Это может быть полезно, например, когда Deckhouse работает в сети с ограничением доступа в интернет.

Адрес веб-интерфейса формируется следующим образом: в шаблоне [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` заменяется на `documentation`.

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс документации будет доступен по адресу `documentation-kube.company.my`.
