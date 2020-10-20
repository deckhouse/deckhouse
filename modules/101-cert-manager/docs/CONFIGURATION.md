---
title: "Модуль cert-manager: конфигурация"
---

Обязательных настроек нет.

## Параметры

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.deckhouse.io/cert-manager":""}` или `{"node-role.deckhouse.io/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"dedicated.deckhouse.io","operator":"Equal","value":"cert-manager"},{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
*  `cloudflareGlobalAPIKey` — Cloudflare Global API key для управления DNS записями (Способ проверки того, что домены указанные в ресурсе Certificate, для которых заказывается сертификат, находятся под управлением `cert-manager` у DNS провайдера Cloudflare. Проверка происходит добавлением специальных TXT записей для домена [ACME DNS01 Challenge Provider](https://cert-manager.io/docs/configuration/acme/dns01/))
*  `cloudflareEmail` — Почтовый ящик проекта, на который выдавались доступы для управления Cloudflare
*  `route53AccessKeyID` — Access Key ID пользователя с необходимыми правами [Amazon Route53 IAM Policy](https://cert-manager.io/docs/configuration/acme/dns01/route53/) для управления доменными записями домена
*  `route53SecretAccessKey` — Secret Access Key пользователя с необходимыми правами для управления доменными записями домена
*  `digitalOceanCredentials` — Access Token от Digital Ocean API, который можно создать в разделе `API`
*  `cloudDNSServiceAccount` — Service Account для [Google Cloud](usage.html#как-заказать-wildcard-сертификат-с-dns-в-google) из того-же проекта с ролью Администратора DNS

### Примеры

```yaml
certManager: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```
