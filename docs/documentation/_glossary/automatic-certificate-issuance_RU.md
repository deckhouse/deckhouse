---
title: Автоматический выпуск сертификатов
description: Автоматическое получение TLS-сертификатов модулем [`cert-manager`](/modules/cert-manager/) либо с помощью объекта Certificate со ссылкой на ClusterIssuer, либо через аннотацию tls-acme на объекте Ingress.
lang: ru
---

С объектом Certificate модуль [`cert-manager`](/modules/cert-manager/) выполняет проверку владения доменом способом, заданным в ClusterIssuer, например `HTTP-01` или `DNS-01`, при необходимости создаёт временный Ingress для проверки и сохраняет сертификат в Secret. Аннотация `tls-acme` на Ingress запрашивает сертификат без отдельного объекта Certificate. Deckhouse Platform поддерживает сертификаты от Let's Encrypt, HashiCorp Vault и Venafi, самоподписанные сертификаты, автоматическое продление и контроль срока действия.

Подробнее в разделе [«Настройка использования TLS-сертификатов»](user/security/tls.html).
