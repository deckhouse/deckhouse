---
title: Шаблон DNS-имён
description: Шаблон DNS-имён кластера из параметра modules.publicDomainTemplate объекта [ModuleConfig](reference/api/cr.html#moduleconfig) global. Модули Deckhouse Platform подставляют вместо `%s` зарезервированное имя и получают DNS-имена своих служебных веб-интерфейсов.
lang: ru
---

Если шаблон не задан, модули не создают Ingress-ресурсы. Домен в шаблоне не должен совпадать с доменом кластера из `clusterDomain` или быть его поддоменом. При wildcard-записи DNS одна запись покрывает все имена. При обычном шаблоне, например `%s-kube.company.my`, для каждого зарезервированного имени нужна A- или CNAME-запись с публичным IP-адресом [Ingress-контроллера](glossary.html#ingress-controller). Шаблон входит в [глобальные настройки](glossary.html#global-settings).

Подробнее в разделе [«Список зарезервированных DNS-имён Deckhouse Platform»](reference/public-domains.html).
