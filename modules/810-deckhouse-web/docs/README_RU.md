---
title: "Модуль deckhouse-web"
webIfaces:
- name: deckhouse
---

Этот модуль создает web-интерфейс с документацией, соответствующей запущенной версии Deckhouse.

Это может быть полезно, например когда Deckhouse работает в сети с ограничением доступа в Интернет.

Адрес web-интерфейса формируется следующим образом: в шаблоне [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` заменяется на `deckhouse`.

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, то web-интерфейс документации будет доступен по адресу `deckhouse-kube.company.my`.
