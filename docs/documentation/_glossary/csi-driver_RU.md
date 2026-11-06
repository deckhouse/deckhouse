---
title: CSI-драйвер
description: Плагин, который управляет постоянными томами хранения в Deckhouse Platform. Входит в состав модулей cloud-provider-* и csi-*, и у всех реализаций одна эталонная архитектура.
lang: ru
---

Container Storage Interface (CSI) — стандартный интерфейс, который унифицирует доступ к хранилищам. У каждого [облачного провайдера](glossary.html#cloud-provider) и [внешнего хранилища](glossary.html#external-storage) своя реализация спецификации. Реализации отличаются составом возможностей и компонентов, но не архитектурой.

Подробнее в разделе [«CSI-драйвер»](architecture/storage/csi-drivers/csi-driver.html).
