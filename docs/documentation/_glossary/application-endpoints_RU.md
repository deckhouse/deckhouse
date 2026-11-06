---
title: Адреса приложения
description: Адреса Ingress приложения, которые Deckhouse Platform публикует в поле `status.urls` объекта [Application](reference/api/cr.html#application). Веб-интерфейс показывает их как ссылки на приложение.
lang: ru
---

Чтобы опубликовать адрес, пакет добавляет Ingress аннотацию `packages.deckhouse.io/application-endpoint-description`: её значение — описание адреса, `"true"` добавляет адрес без описания, `"false"` исключает Ingress. Deckhouse Platform строит URL для каждой пары `host` и `path` в правилах Ingress, со схемой `https`, если хост указан в `spec.tls`, и обновляет `status.urls` [экземпляра приложения](glossary.html#application-instance) после каждого успешного применения шаблонов.

Подробнее в разделе [«Шаблоны»](architecture/marketplace/templates.html).
