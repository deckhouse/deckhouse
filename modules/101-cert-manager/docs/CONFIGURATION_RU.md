---
title: "Модуль cert-manager: настройки"
---

Обязательных настроек нет.

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано или указано `false` — будет [использоваться автоматика](../../#выделение-узлов-под-определенный-вид-нагрузки).
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано или указано `false` — будет [использоваться автоматика](../../#выделение-узлов-под-определенный-вид-нагрузки).
*  `cloudflareGlobalAPIKey` — Cloudflare Global API key для управления DNS записями (Способ проверки того, что домены указанные в ресурсе Certificate, для которых заказывается сертификат, находятся под управлением `cert-manager` у DNS провайдера Cloudflare. Проверка происходит добавлением специальных TXT записей для домена [ACME DNS01 Challenge Provider](https://cert-manager.io/docs/configuration/acme/dns01/))
*  `cloudflareEmail` — Почтовый ящик проекта, на который выдавались доступы для управления Cloudflare
*  `route53AccessKeyID` — Access Key ID пользователя с необходимыми правами [Amazon Route53 IAM Policy](https://cert-manager.io/docs/configuration/acme/dns01/route53/) для управления доменными записями домена
*  `route53SecretAccessKey` — Secret Access Key пользователя с необходимыми правами для управления доменными записями домена
*  `digitalOceanCredentials` — Access Token от Digital Ocean API, который можно создать в разделе `API`
*  `cloudDNSServiceAccount` — Service Account для [Google Cloud](usage.html#заказ-wildcard-сертификата-с-dns-в-google) из того-же проекта с ролью Администратора DNS
