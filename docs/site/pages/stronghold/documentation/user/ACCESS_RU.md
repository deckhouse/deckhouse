---
title: "Настройка доступа к проекту"
permalink: ru/stronghold/documentation/user/access.html
lang: ru
---

Чтобы настроить доступ к своему проекту из командной строки в Deckhouse Stronghold, выполните следующее:

1. Установите [утилиту `d8`](/products/kubernetes-platform/documentation/v1/cli/d8/).
2. Установите адрес вашего сервера Stronghold:

   ```shell
   export STRONGHOLD_ADDR=https://stronghold.domain.my
   ```

3. Авторизуйтесь в Stronghold с помощью следующей команды:

   ```shell
   d8 stronghold login -path=oidc_deckhouse -method=oidc -no-print
   ```

4. Далее используйте следующий формат команд для управления объектами:

   ```shell
   d8 stronghold <command>
   ```
