---
title: "Глобальные настройки"
permalink: ru/deckhouse-configure-global.html
lang: ru
---

Глобальные настройки Deckhouse хранятся в ресурсе `ModuleConfig/global`. Глобальные настройки можно рассматривать как специальный [модуль](./#настройка-модуля) `global`, который нельзя отключить.

> В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон, с учетом которого некоторые модули Deckhouse создают Ingress-ресурсы. Чтобы получить к ним доступ вы должны настроить ваш DNS, либо добавить DNS-записи локально (например в файле `/etc/hosts` для Linux).
>
> Если у вас нет возможности заводить wildcard-записи DNS, для целей тестирования вы можете воспользоваться сервисом [sslip.io](https://sslip.io) или аналогами.

## Параметры

{{ site.data.schemas.global.config-values | format_configuration }}
