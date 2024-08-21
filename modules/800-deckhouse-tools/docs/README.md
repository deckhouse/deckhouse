---
title: "The deckhouse-tools module"
webIfaces:
- name: deckhouse-tools
description: "The deckhouse-tools Deckhouse module provides a web interface in the cluster for downloading Deckhouse utilities (Deckhouse CLI)"
---

Этот модуль создает веб-интерфейс со ссылками на скачивание утилит Deckhouse (в настоящее время – [Deckhouse CLI](../../deckhouse-cli/) под различные операционные системы).

Адрес веб-интерфейса формируется в соответствии с шаблоном [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse (ключ `%s` заменяется на `tools`).

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс будет доступен по адресу `tools-kube.company.my`.
