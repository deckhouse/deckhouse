---
title: "Архитектура режима Direct"
permalink: ru/architecture/registry-direct-mode.html
lang: ru
---

В режиме `Direct` запросы к реестру образов обрабатываются напрямую, без промежуточного кеширования.

Перенаправление запросов к реестру от CRI осуществляется при помощи его настроек, которые указываются в конфигурации `containerd`.

В случае таких компонентов, как `operator-trivy`, `image-availability-exporter`, `deckhouse-controller` и ряда других, обращающихся к registry напрямую, запросы будут идти через in-cluster proxy, расположенный на master-узлах.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](../images/registry-module/direct-ru.png)

Подробнее о режиме `Direct` — в разделе [«Использование внутреннего реестра образов контейнеров»](../admin/configuration/registry/internal-registry.html).
