---
title: Схема размещения
description: Схема размещения узлов в инфраструктуре облачного провайдера. Влияет на сеть, наличие публичных IP-адресов, маршрутизацию исходящего трафика и способ доступа к узлам.
lang: ru
---

Схема размещения задаётся в параметре `layout` [ModuleConfig](reference/api/cr.html#moduleconfig) модуля [облачного провайдера](glossary.html#cloud-provider). Доступные схемы зависят от провайдера, например `Standard`, `WithoutNAT` и `WithNATInstance` в Yandex Cloud.

В некоторых схемах узлы не имеют публичных IP-адресов, и доступ к master-узлу выполняется через [bastion-хост](glossary.html#bastion-host).

Подробнее в разделе [«Интеграция с облачными провайдерами»](admin/integrations/public/overview.html).
