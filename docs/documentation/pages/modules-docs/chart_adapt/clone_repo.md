---
title: "Сделайте форк или скопируйте шаблон репозитория с модулем"
permalink: en/modules-docs/chart-adapt/clone-repo/
---

## Сделайте форк или скопируйте шаблон репозитория с модулем

Для удобства создания модулей команда Deckhouse подготовила репозиторий для быстрого старта. Внутри репозитория находится пример минимального модуль со всеми возможными функциями. Мы будем использовать этот репозиторий как основу.

1. Сделайте форк шаблона для модуля в Gitlab (cсылка на репозиторий - [fox.flant.com/deckhouse/modules/template](https://fox.flant.com/deckhouse/modules/template)):

   ![Fork](../../../images/modules-docs/fork.png)

1. Склонируйте его.

   ```sh
   git clone git@fox.flant.com:***/hello-world-module.git hello-world-module \
     && cd hello-world-module
   ```

   > **NOTE:** Подставьте свой адрес для команды git clone.
