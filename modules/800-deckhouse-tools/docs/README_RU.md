---
title: "Модуль deckhouse-tools"
webIfaces:
- name: deckhouse-tools
description: "Веб-интерфейс для скачивания утилиты Deckhouse CLI (d8) под различные операционные системы."
---

Этот модуль создает веб-интерфейс со ссылками на скачивание утилит Deckhouse (в настоящее время – [Deckhouse CLI](../../deckhouse-cli/) под различные операционные системы).

Адрес веб-интерфейса формируется в соответствии с шаблоном [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse (ключ `%s` заменяется на `tools`).

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс будет доступен по адресу `tools-kube.company.my`.
