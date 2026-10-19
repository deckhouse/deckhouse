---
title: Список зарезервированных DNS-имён Deckhouse Platform
permalink: ru/reference/public-domains.html
description: Список зарезервированных DNS-имён, которые использует Deckhouse Platform. Генератор данных для hosts-файла
lang: ru
editionsNotAvailable: [cse]
search: публичные домены DP
---

На странице перечислены зарезервированные DNS-имена, которые использует Deckhouse Platform (DP). Имена приведены в [таблице ниже](#список-зарезервированных-dns-имён).

Список нужен, если шаблон DNS-имён кластера ([параметр `modules.publicDomainTemplate`](api/global.html#parameters-modules-publicdomaintemplate) ресурса ModuleConfig `global`) — не wildcard-шаблон (например, `%s-kube.company.my`). В этом случае добавьте для каждого имени A- или CNAME-запись. Значение записи — публичный IP-адрес Ingress-контроллера (балансировщика HTTP/HTTPS-трафика).

Чтобы получить команду для добавления DNS-имён в локальный hosts-файл, используйте генератор данных для hosts-файла.

## Генератор данных для hosts-файла

Чтобы обращаться к сервисам кластера без DNS-записей (например, с рабочей станции), добавьте в hosts-файл записи для нужных имён из [таблицы ниже](#список-зарезервированных-dns-имён). Укажите в генераторе шаблон DNS-имён (из параметра [`modules.publicDomainTemplate`](api/global.html#parameters-modules-publicdomaintemplate) ресурса ModuleConfig `global`) и IP-адрес Ingress-контроллера. Генератор выдаст готовую команду:

{% include public_domains_form.liquid %}

## Список зарезервированных DNS-имён

В таблице перечислены зарезервированные DNS-имена DP и сервисы, которые их используют.

Когда добавляете A- или CNAME-записи, подставьте имя из таблицы вместо `%s` в `publicDomainTemplate`. Например, для шаблона `%s.example.com` сервис `grafana` получает имя `grafana.example.com`, для `%s-kube.company.my` — `grafana-kube.company.my`.

{% include domains_list.liquid %}
