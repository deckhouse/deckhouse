---
title: Управляющий кластер
description: Кластер Deckhouse Platform с установленным модулем [`commander`](/modules/commander/). Он создаёт однотипные кластеры Deckhouse Platform и управляет их конфигурацией и жизненным циклом.
lang: ru
---

Модуль [`commander`](/modules/commander/) реализует веб-приложение для управления кластерами, ему нужен экземпляр PostgreSQL, развёрнутый внутри или вне кластера. Кластеры под его управлением работают с модулем [`commander-agent`](/modules/commander-agent/) и называются [управляемыми](glossary.html#managed-cluster).

Подробнее в разделе [«Управление кластерами»](architecture/deckhouse/commander.html).
