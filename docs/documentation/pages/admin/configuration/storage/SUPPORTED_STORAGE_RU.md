---
title: "Настройка хранилищ"
permalink: ru/admin/configuration/storage/supported-storage.html
description: "Настройка поддерживаемых типов хранилищ в Deckhouse Kubernetes Platform. Конфигурация StorageClass, управление хранилищами и настройка различных типов хранилищ."
lang: ru
search: default storage class, supported-storagestorage configuration, storage setup, storage management, storage types, настройка хранилищ, конфигурация хранилища, управление хранилищем, типы хранилищ, поддерживаемые хранилища
---

Настройка хранилищ происходит в несколько шагов, которые зависят от выбранного [типа хранилища](../storage/#поддерживаемые-типы-хранилищ). Основные этапы настройки:

- Включение и конфигурирование соответствующих модулей.
- Создание групп томов (Volume Groups).
- Подготовка и создание объектов StorageClass, их последующее назначение и использование.

У каждого типа хранилища могут быть свои специфические требования и нюансы конфигурации, описанные в соответствующих руководствах.

## Создание StorageClass

Для создания объектов StorageClass необходимо подключить одно или несколько хранилищ, которые управляют ресурсами PersistentVolume. Созданные объекты StorageClass можно использовать для организации виртуальных дисков и образов. Подробнее о создании и использовании StorageClass можно узнать в соответствующих разделах документации по каждому [типу хранилища](../storage/#поддерживаемые-типы-хранилищ).

## Назначение StorageClass по умолчанию

StorageClass по умолчанию используется в случаях, когда при создании ресурсов PersistentVolumeClaim явно не указан класс хранения. Это упрощает процесс создания и использования хранилищ, избегая необходимости каждый раз указывать класс вручную.

Чтобы задать StorageClass по умолчанию, укажите нужный класс хранения в [глобальной конфигурации](../../../reference/api/global.html#parameters-defaultclusterstorageclass). Пример команды:

```shell
# Укажите имя своего объекта StorageClass.
DEFAULT_STORAGE_CLASS=replicated-storage-class
d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
