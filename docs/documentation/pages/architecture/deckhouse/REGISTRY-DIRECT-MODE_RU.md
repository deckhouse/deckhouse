---
title: "Архитектура режима Direct"
permalink: ru/architecture/registry-direct-mode.html
lang: ru
search: режим Direct, архитектура registry, внутренний registry
description: Архитектура режима Direct модуля registry в DKP — обработка запросов к хранилищу образов без промежуточного кэширования.
relatedLinks:
  - url: /modules/registry/
---

В режиме `Direct` модуля `registry` запросы к хранилищу образов контейнеров обрабатываются напрямую, без промежуточного кеширования.

Перенаправление запросов к хранилищу образов от CRI осуществляется при помощи его настроек, которые указываются в конфигурации `containerd`.

В случае таких компонентов, как [`operator-trivy`](/modules/operator-trivy/), `image-availability-exporter`, `deckhouse-controller` и ряда других, обращающихся к хранилищу образов напрямую, запросы будут идти через внутрикластерный прокси-сервер, расположенный на master-узлах.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![Режим Direct модуля registry](../images/registry-module/direct-ru.png)

Подробнее о режиме `Direct` читайте в [разделе об управлении внутренним хранилищем образов](../admin/configuration/registry/internal.html).
