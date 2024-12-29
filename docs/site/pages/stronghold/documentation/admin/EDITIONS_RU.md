---
title: "Редакции платформы"
permalink: ru/stronghold/documentation/admin/editions.html
lang: ru
---

Deckhouse Stronghold поставляется в редакциях Standart Edition (SE), Standart Edition Plus (SE+), Enterprise Edition (EE). Deckhouse Stronghold CSE, имеющая сертификат ФСТЭК, готовится к выходу в ближайшее время. Редакции отличаются набором возможностей и уровнем доступной поддержки.

Краткое сравнение ключевых возможностей и особенностей редакций:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% assign not_supported = '<img src="/images/icons/not_supported.svg">' %}
{% assign supported = '<img src="/images/icons/supported.svg">' %}

| Возможности                                                     | SE                  | SE+               | EE |
|-----------------------------------------------------------------|---------------------|-------------------|----|
| Поддержка российских ОС                                         | {{ supported }}     | {{ supported }} | {{ supported }} |
| Развертывание в закрытом контуре                                | {{ supported }}     | {{ supported }} | {{ supported }} |
| Сетевые политики (микросегментация)                             | {{ supported }}     | {{ supported }} | {{ supported }} |
| Расширенный мониторинг                                          | {{ supported }}     | {{ supported }} | {{ supported }} |
| Управление балансировкой трафика                                | {{ supported }}     | {{ supported }} | {{ supported }} |
| Интерфейс администратора                                        | {{ supported }}     | {{ supported }} | {{ supported }} |
| Режим высокой доступности (HA)                                  | {{ supported }}     | {{ supported }} | {{ supported }} |
| Запрет на запуск контейнеров с уязвимостями                     | {{ not_supported }} | {{ not_supported }} | {{ supported }} |
| Поиск угроз безопасности                                        | {{ not_supported }} | {{ not_supported }} | {{ supported }} |
| Сканирование образов в runtime на уязвимости                    | {{ not_supported }} | {{ not_supported }} | {{ supported }} |
| [Техподдержка «Стандарт»](https://deckhouse.ru/tech-support/)   | {{ supported }}     | {{ supported }} | {{ supported }} |
| [Техподдержка «Стандарт +»](https://deckhouse.ru/tech-support/) | {{ supported }}     | {{ supported }} | {{ supported }} |
