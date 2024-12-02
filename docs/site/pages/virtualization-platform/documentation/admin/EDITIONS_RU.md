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

| Возможности                                                     | CE                  | EE |
|-----------------------------------------------------------------|---------------------|----|
| Поддержка российских ОС                                         | {{ not_supported }} | {{ supported }} |
| Развертывание в закрытом контуре                                | {{ not_supported }} | {{ supported }} |
| Сетевые политики (микросегментация)                             | {{ supported }}     | {{ supported }} |
| Расширенный мониторинг                                          | {{ supported }}     | {{ supported }} |
| Управление балансировкой трафика                                | {{ supported }}     | {{ supported }} |
| Поддержка NFS                                                   | {{ supported }}     | {{ supported }} |
| Встроенный SDS                                                  | {{ supported }}     | {{ supported }} |
| Поддержка аппаратных СХД                                        | {{ not_supported }} | {{ supported }} |
| Общедоступный LUN                                               | {{ coming_soon }}   | {{ coming_soon }} |
| Интерфейс администратора                                        | {{ not_supported }} | {{ supported }} |
| Изменение параметров ВМ без ее остановки                        | {{ not_supported }} | {{ supported }} |
| Режим высокой доступности (HA) виртуальных машин                | {{ coming_soon }}   | {{ coming_soon }} |
| Катастрофоустойчивость (межкластерная репликация)               | {{ coming_soon }}   | {{ coming_soon }} |
| [Техподдержка «Стандарт»](https://deckhouse.ru/tech-support/)   | {{ not_supported }} | {{ supported }}  |
| [Техподдержка «Стандарт +»](https://deckhouse.ru/tech-support/) | {{ not_supported }} | {{ supported }}  |
