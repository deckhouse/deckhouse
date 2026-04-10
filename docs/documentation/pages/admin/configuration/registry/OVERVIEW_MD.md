---
title: Обзор
permalink: ru/admin/configuration/registry/
description: "Как устроена работа с registry и редакциями в Deckhouse Kubernetes Platform."
lang: ru
search: registry, container registry, editions, Deckhouse registry, хранилище образов, редакции
---

В этом разделе собраны инструкции по работе с registry и редакциями Deckhouse Kubernetes Platform.

## Что можно настроить

### Хранилище образов компонентов DKP

Этот сценарий нужен, если вы хотите настроить, откуда платформа берёт свои образы.

Выберите подходящий раздел:

- [Кластер, управляемый DKP](../dkp_component/managing-interaction) — если кластер полностью управляется DKP и вы настраиваете registry через модуль `registry`.
- [Managed Kubernetes: сторонний registry](../dkp_component/third-party) — если кластер работает в Managed Kubernetes и вы меняете registry через `helper change-registry`.
- [Восстановление подключения к registry](../dkp_component/restore-token) — если нужно восстановить доступ к registry компонентов платформы.

### Пользовательское хранилище образов

Этот сценарий нужен, если вы хотите хранить внутри кластера образы собственных приложений.

Доступны два варианта:

- [Payload registry](../custom_image_storage/payload-registry) — развёртывание пользовательского OCI-совместимого registry с помощью модуля `payload-registry`.
- [Внутренний registry](../custom_image_storage/internal) — работа со встроенным registry для внутренних сценариев.

### Редакции

Если нужно сменить редакцию платформы, используйте раздел [Переключение редакций](../switching-editions).

## Как выбрать нужную инструкцию

Используйте эту схему:

- Нужно настроить registry, из которого DKP скачивает свои образы?
  - Кластер полностью управляется DKP → [Кластер, управляемый DKP](../dkp_component/managing-interaction)
  - Кластер работает в Managed Kubernetes → [Managed Kubernetes: сторонний registry](../dkp_component/third-party)
- Нужно хранить образы приложений внутри кластера? → [Payload registry](../custom_image_storage/payload-registry) или [Внутренний registry](../custom_image_storage/internal)
- Нужно сменить редакцию DKP? → [Переключение редакций](../switching-editions)

## На что обратить внимание

Перед началом работ проверьте:

- какой у вас тип кластера: полностью управляемый DKP или Managed Kubernetes;
- что именно вы настраиваете: registry компонентов DKP или registry для приложений;
- есть ли ограничения по вашей редакции;
- готовы ли вы к возможному перезапуску компонентов, если меняете registry платформы.

## Что дальше

Если вы не уверены, с чего начать, обычно достаточно ответить на два вопроса:

1. Где работает кластер — в полностью управляемом DKP или в Managed Kubernetes?
2. Вы хотите настроить registry платформы или registry для приложений?

После этого можно перейти к нужной инструкции из списка выше.
