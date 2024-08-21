---
title: "Модуль deckhouse-tools"
webIfaces:
- name: deckhouse-tools
---

Этот модуль создает веб-интерфейс с ссылками на скачивание утилит Deckhouse (в настоящее время d8-cli под различные операционные системы).

Это может быть полезно, например, когда Deckhouse работает в сети с ограничением доступа в интернет.

Адрес веб-интерфейса формируется следующим образом: в шаблоне [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` заменяется на `tools`.

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс будет доступен по адресу `tools-kube.company.my`.
