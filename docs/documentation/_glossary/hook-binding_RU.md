---
title: Биндинг хука
description: Событие, при котором срабатывает хук, указанное в конфигурации хука. Хук может сработать по событию в Kubernetes, по расписанию, при старте, до или после применения Helm-шаблонов.
lang: ru
---

Биндинги [хука модуля](glossary.html#module-hook): `onStartup`, `beforeHelm`, `afterHelm`, `kubernetes` и `schedule`. У глобальных хуков есть также `beforeAll` и `afterAll`. Хуки [пакета Marketplace](glossary.html#marketplace-package) используют те же виды биндингов, а поле `Queue` задаёт отдельную очередь для биндингов `Schedule` и `Kubernetes`. Хуки с одинаковым биндингом выполняются в порядке значения `Order`.

Подробнее в разделе [«Хуки»](architecture/marketplace/hooks.html).
