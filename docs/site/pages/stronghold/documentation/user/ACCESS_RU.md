---
title: "Настройка доступа к проекту"
permalink: ru/stronghold/documentation/user/access.html
lang: ru
---

Для работы с Stronghold из командной строки выполните следующие действия:

1. Установите утилиту [d8](/products/kubernetes-platform/documentation/v1/cli/d8/).
2. Установите адрес вашего сервера Stonghold `export STRONGHOLD_ADDR=https://stronghold.domain.my`
3. Авторизуйтесь с помощью команды `d8 stronghold login -path=oidc_deckhouse -method=oidc -no-print`
4. Далее используйте следующий формат команды для управления объектами: `d8 stronghold <command>`.
