---
title: "Редакции платформы"
permalink: ru/virtualization-platform/documentation/admin/editions.html
lang: ru
---

Deckhouse Virtualization Platform поставляется в редакциях Community Edition (CE) и Enterprise Edition (EE). Deckhouse Virtualization Platform CSE, имеющая сертификат ФСТЭК, готовится к выходу в ближайшее время. Редакции DVP отличаются набором возможностей и уровнем доступной поддержки.

Краткое сравнение ключевых возможностей и особенностей редакций:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% assign not_supported = '<img src="/images/icons/not_supported.svg">' %}
{% assign supported = '<img src="/images/icons/supported.svg">' %}

| Возможности                                                     | CE                   | EE               |
|-----------------------------------------------------------------|----------------------|------------------|
| Декларативное создание любых ресурсов (GitOps ready)            | {{ supported }}      | {{ supported }}  |
| Масштабирование до 1000 узлов и 50 000 ВМ                       | {{ supported }}      | {{ supported }}  |
| Поддержка российских ОС на гипервизоре                          | {{ not_supported }}  | {{ supported }}  |
| Режим обслуживания гипервизоров                                 | {{ supported }}      | {{ supported }}  |
| Высокая доступность ВМ при отказе узла гипервизора              | {{ supported }}      | {{ supported }}  |
| **Планирование ресурсов**                                       |                      |                  |
| Квотирование ресурсов на уровне проектов                        | {{ supported }}      | {{ supported }}  |
| Политики сайзинга виртуальных машин (VirtualMachineClass)       | {{ supported }}      | {{ supported }}  |
| Унификация CPU-инструкций на гипервизорах (VirtualMachineClass) | {{ supported }}      | {{ supported }}  |
| **Возможности управления**                                      |                      |                  |
| Веб-интерфейс администратора                                    | {{ coming_soon }}    | {{ supported }}  |
| Управление через CLI и доступ через API                         | {{ supported }}      | {{ supported }}  |
| Импорт образов и дисков ВМ (qcow, vmdk, raw, vdi)     | {{ supported }}      | {{ supported }}  |
| Общие и проектные образы для создания виртуальных машин         | {{ supported }}      | {{ supported }}  |
| Кастомизация ОС ВМ при первом запуске                           | {{ supported }}      | {{ supported }}  |
| Живая миграция виртуальных машин без простоя                    | {{ supported }}      | {{ supported }}  |
| Консистентные снимки дисков                                     | {{ supported }}      | {{ supported }}  |
| Добавление и изменение размеров дисков ВМ без перезагрузки      | {{ supported }}      | {{ supported }}  |
| Политики запуска ВМ                                             | {{ supported }}      | {{ supported }}  |
| Правила размещения ВМ (affinity/antiaffinity)                   | {{ supported }}      | {{ supported }}  |
| **Хранение данных**                                             |                      |                  |
| Встроенный SDS                                                  | {{ supported }}      | {{ supported }}  |
| Поддержка СХД с использованием API (Yadro, Huawei, HPE)         | {{ not_supported }}  | {{ supported }}  |
| Универсальная поддержка аппаратных СХД (SCSI-generic)           | {{ not_supported }}  | {{ supported }}  |
| Поддержка NFS                                                   | {{ supported }}      | {{ supported }}  |
| Поддержка сторонних SDS (Ceph)                                  | {{ supported }}      | {{ supported }}  |
| **Сетевые возможности (SDN)**                                   |                      |                  |
| Микросегментация на основе сетевых политик                      | {{ supported }}      | {{ supported }}  |
| Встроенный балансировщик нагрузки                               | {{ supported }}      | {{ supported }}  |
| Внешний балансировщик нагрузки на базе MetalLB                  | {{ not_supported }}  | {{ supported }}  |
| Балансировщик нагрузки с активными healthcheck                  | {{ not_supported }}  | {{ supported }}  |
| Управление статическими маршрутами                              | {{ not_supported }}  | {{ supported }}  |
| Egress Gateway                                                  | {{ not_supported }}  | {{ supported }}  |
| **Безопасность**                                                |                      |                  |
| Мультиарендность на базе проектов                               | {{ supported }}      | {{ supported }}  |
| Гибкая ролевая модель доступа                                   | {{ supported }}      | {{ supported }}  |
| Интеграция с внешними провайдерами аутентификации (LDAP, OIDC)  | {{ supported }}      | {{ supported }}  |
| Миграция ВМ с шифрованием (Data in Transit encryption)          | {{ supported }}      | {{ supported }}  |
| Управление сертификатами                                        | {{ supported }}      | {{ supported }}  |
| Развёртывание в закрытом контуре                                | {{ not_supported }}  | {{ supported }}  |
| **Мониторинг**                                                  |                      |                  |
| Встроенный мониторинг и логирование инфраструктуры и ВМ         | {{ supported }}      | {{ supported }}  |
| Отправка метрик и логов во внешние источники                    | {{ supported }}      | {{ supported }}  |
| **Поддержка**                                                   |                      |                  |
| Поддержка сообщества                                            | {{ supported }}      | {{ supported }}  |
| [Техподдержка «Стандарт» (8/5)](/tech-support/)                 | {{ not_supported }}  | {{ supported }}  |
| [Техподдержка «Стандарт+» (24/7)](/tech-support/)               | {{ not_supported }}  | {{ supported }}  |
