---
title: "Редакции"
permalink: ru/stronghold/documentation/about/editions.html
lang: ru
---

Deckhouse Stronghold поставляется в редакциях Community Edition (CE), Enterprise Edition (EE). Редакция Certified Security Edition (CSE), сертифицированная ФСТЭК России для сред с повышенными требованиями к информационной безопасности, **ожидается в 2025 году**.

Deckhouse Stronghold CE доступен для использования в любой редакции Deckhouse Kubernetes Platform (DKP).

Deckhouse Stronghold EE и Deckhouse Stronghold CSE лицензируются отдельно. Deckhouse Stronghold EE доступен для использования в любой **коммерческой редакции** DKP. Deckhouse Stronghold CSE будет доступен для использования только в редакции DKP CSE.

Краткое сравнение ключевых возможностей и особенностей редакций Deckhouse Stronghold:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_ce %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_ce[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_commercial %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_commercial[page.lang] }}" aria-expanded="false">{% endcapture %}

{% assign not_supported = '<img src="/images/icons/not_supported_v2.svg">' %}
{% assign supported = '<img src="/images/icons/supported_v2.svg">' %}

| Возможности                                                                                                      | CE                                               | EE                                                              | CSE **(ожидается в 2025 году)**                |
|------------------------------------------------------------------------------------------------------------------|--------------------------------------------------|-----------------------------------------------------------------|------------------------------------------------|
| Безопасное управление жизненным циклом секретов (хранение, создание, доставка, отзыв и ротация)                  | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Возможность использования инструментов автоматизации IaC (Ansible, Terraform)                                      | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка методов аутентификации                                                                                 | JWT, OIDC, Kubernetes, LDAP, Token               | JWT, OIDC, Kubernetes, LDAP, Token                     | JWT, OIDC, Kubernetes, LDAP, Token    |
| Поддержка механизмов секретов KV, Kubernetes, Database, SSH, PKI                                                      | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка российских ОС ([подробнее...](/products/kubernetes-platform/documentation/v1/supported_versions.html)) | РЕД ОС, ALT Linux, Astra Linux Special Edition, **РОСА Сервер** | РЕД ОС, ALT Linux, Astra Linux Special Edition, **РОСА Сервер** | РЕД ОС, ALT Linux, Astra Linux Special Edition |
| Развёртывание в закрытом контуре                                                                                 | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Веб-интерфейс                                                                                                    | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Управление ролями и политиками доступа через веб-интерфейс                                                       | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка пространств имён (namespaces)                                                                          | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Встроенное автоматическое распечатывание хранилища (auto unseal) без использования внешних сервисов и KMS                                                 | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Репликация данных                                                                                                | {{ not_supported }}                                  | KV1/KV2                                                         | KV1/KV2                                        |
| Автоматическое создание резервных копий по заданному расписанию                                                  | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка аудит-логирования                                                                                      | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Возможность поставки в виде исполняемого файла (standalone)                                                      | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Сертификат соответствия требованиям Приказа ФСТЭК России №76 по 4 уровню доверия                                 | {{ not_supported }}                                  | {{ not_supported }}                                             | {{ supported }}                                |
| Возможность запуска в DKP CE                                            | {{ supported }}                                  | {{ not_supported }}                                             | {{ not_supported }}                            |
| [Гарантийная техническая поддержка](https://deckhouse.ru/tech-support/)                                          | {{ techsupport_notice_ce }}                                  | {{ techsupport_notice_commercial }}                             | {{ techsupport_notice_commercial }}            |
| [Техподдержка «Стандарт»](https://deckhouse.ru/tech-support/)                                                    | {{ techsupport_notice_ce }}                                  | {{ techsupport_notice_commercial }}                             | {{ techsupport_notice_commercial }}            |
| [Техподдержка «Стандарт +»](https://deckhouse.ru/tech-support/)                                                  | {{ techsupport_notice_ce }}                                  | {{ techsupport_notice_commercial }}                             | {{ techsupport_notice_commercial }}            |
