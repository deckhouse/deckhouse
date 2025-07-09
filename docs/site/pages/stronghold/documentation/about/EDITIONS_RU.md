---
title: "Редакции платформы"
permalink: ru/stronghold/documentation/about/editions.html
lang: ru
---

Deckhouse Stronghold доступен в редакциях Community Edition (CE), Enterprise Edition (EE) и редакции Certified Security Edition (CSE), сертифицированной ФСТЭК России для сред с повышенными требованиями к информационной безопасности.

Deckhouse Stronghold Community Edition доступен для использования в любой редакции Deckhouse Kubernetes Platform.

Deckhouse Stronghold Enterprise Edition и Certified Security Edition лицензируются отдельно. Deckhouse Stronghold Enterprise Edition доступен для использования в любой коммерческой редакции Deckhouse Kubernetes Platform. Deckhouse Stronghold Certified Security Edition доступен для использования только в редакции Deckhouse Kubernetes Platform Certified Security Edition.

Для подробного сравнения возможностей перейдите к разделу [Сравнение редакций Deckhouse Kubernetes Platform](../../../kubernetes-platform/documentation/v1/revision-comparison.html).

Краткое сравнение ключевых возможностей и особенностей редакций:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_ce %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_ce[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_ee %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_ee[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_cse %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_cse[page.lang] }}" aria-expanded="false">{% endcapture %}

{% assign not_supported = '<img src="/images/icons/not_supported_v2.svg">' %}
{% assign supported = '<img src="/images/icons/supported_v2.svg">' %}

| Возможности                                                                                                      | CE                                               | EE                                                              | CSE                                            |
|------------------------------------------------------------------------------------------------------------------|--------------------------------------------------|-----------------------------------------------------------------|------------------------------------------------|
| Безопасное управление жизненным циклом секретов (хранение, создание, доставка, отзыв и ротация)                  | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Возможность использования инструментов автоматизации IaC Ansible, Terraform                                      | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка методов аутентификации                                                                                 | JWT, OIDC, Kubernetes, LDAP, Token               | JWT, OIDC, Kubernetes, LDAP, Token, **TLS**                     | JWT, OIDC, Kubernetes, LDAP, Token, **TLS**    |
| Поддержка Secret Engines KV, Kubernetes, Database, SSH, PKI                                                      | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка российских ОС ([подробнее...](/products/kubernetes-platform/documentation/v1/supported_versions.html)) | РЕД ОС, ALT Linux, Astra Linux Special Edition, **РОСА Сервер** | РЕД ОС, ALT Linux, Astra Linux Special Edition, **РОСА Сервер** | РЕД ОС, ALT Linux, Astra Linux Special Edition |
| Развёртывание в закрытом контуре                                                                                 | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Веб-интерфейс                                                                                                    | {{ supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Управление ролями и политиками доступа через веб-интерфейс                                                       | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка пространств имён (namespaces)                                                                          | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Встроенное автоматическое распечатывание (auto unseal) хранилища                                                 | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Репликация данных                                                                                                | {{ not_supported }}                                  | KV1/KV2                                                         | KV1/KV2                                        |
| Автоматическое создание резервных копий по заданному расписанию                                                  | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Поддержка аудит-логирования                                                                                      | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Возможность поставки в виде исполняемого файла (Standalone)                                                      | {{ not_supported }}                                  | {{ supported }}                                                 | {{ supported }}                                |
| Сертификат соответствия требованиям Приказа ФСТЭК России №76 по 4 уровню доверия                                 | {{ not_supported }}                                  | {{ not_supported }}                                             | {{ supported }}                                |
| Возможность запуска в Deckhouse Kubernetes Platform Community Edition                                            | {{ supported }}                                  | {{ not_supported }}                                             | {{ not_supported }}                            |
| [Гарантийная техническая поддержка](https://deckhouse.ru/tech-support/)                                          | {{ techsupport_notice_ce }}                                  | {{ techsupport_notice_ee }}                                     | {{ techsupport_notice_cse }}                   |
| [Техподдержка «Стандарт»](https://deckhouse.ru/tech-support/)                                                    | {{ techsupport_notice_ce }}                                  | {{ techsupport_notice_ee }}                                     | {{ techsupport_notice_cse }}                   |
| [Техподдержка «Стандарт +»](https://deckhouse.ru/tech-support/)                                                  | {{ techsupport_notice_ce }}                                  | {{ techsupport_notice_ee }}                                     | {{ techsupport_notice_cse }}                   |
