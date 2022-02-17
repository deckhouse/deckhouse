---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

Глобальные настройки Deckhouse хранятся в параметре `global` [конфигурации Deckhouse](./#deckhouse-configuration).

> В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон, с учетом которого некоторые модули Deckhouse создают Ingress-ресурсы. Чтобы получить к ним доступ вы должны настроить ваш DNS, либо добавить DNS-записи локально (например в файле `/etc/hosts` для Linux).
>
> You can use the nip.io service (or similar) for testing if wildcard DNS records are unavailable to you for some reason.
> Pay attention to some [nuances](./#deckhouse-configuration) of ConfigMap `deckhouse`.

## Parameters

{{ site.data.schemas.global.config-values | format_configuration }}
