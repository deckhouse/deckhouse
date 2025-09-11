---
title: "Руководство по API автоматического резервного копирования"
permalink: ru/stronghold/documentation/user/auto-snapshot.html
lang: ru
description: "Руководство администратора по работе с API автоматического резервного копирования Deckhouse Stronghold."
---

Deckhouse Stronghold позволяет настроить расписание для автоматического создания резервных копий хранилища секретов.
Поскольку Stronghold хранит данные на диске в зашифрованном виде, резервная копия также содержит только зашифрованные данные.
Для получения доступа к данным необходимо развернуть резервную копию в кластере Stronghold и выполнить процедуру распечатывания хранилища.

Резервные копии можно сохранять на локальный диск в выбранную директорию или в S3-совместимое хранилище.

Управлять настройками резервных копий и просматривать их статус можно через API, Stronghold CLI и веб-интерфейс.

## Создание или обновление конфигурации автоматического резервного копирования

| Метод | Путь |
|-------|------|
| POST  | `/sys/storage/raft/snapshot-auto/config/:name` |

Для работы с данным методом API потребуются права sudo.

### Описание параметров

<div class="table__styling--container"></div>

| Параметр | Тип | Обязательный | Значение по умолчанию | Описание |
|----------|-----|--------------|-----------------------|----------|
| `name` | Cтрока | Да | — | Имя конфигурации, которую необходимо создать или изменить. |
| `interval` | Целое число или строка | Да | — | Интервал между резервными копиями. Может задаваться в секундах или в формате Go duration (например, `24h`). |
| `retain` | Целое число | Нет | `3` | Количество резервных копий, которые должны храниться. При превышении этого числа самые старые резервные копии удаляются. |
| `path_prefix` | Неизменяемая строка | Да | — | Если в параметре `storage_type` выбрано локальное хранилище, здесь указывается директория хранения резервных копий. Если выбрано облачное хранилище, здесь указывается bucket-префикс (начальный `/` игнорируется, последующие `/` необязательны). |
| `file_prefix` | Неизменяемая строка | Нет | `stronghold-snapshot` | Префикс имени файла или объекта резервной копии в пределах директории или bucket, заданного в `path_prefix`. |
| `storage_type` | Неизменяемая строка | Да | — | Тип хранилища резервных копий: `local` (локальное) или `aws-s3` (облачное). Остальные параметры ниже зависят от выбранного типа хранилища. |

#### Дополнительные параметры для локального хранилища

<div class="table__styling--container"></div>

| Параметр | Тип | Обязательный | Значение по умолчанию | Описание |
|----------|-----|--------------|-----------------------|----------|
| `local_max_space` | Целое число | Нет | `0` | Максимальный объём (в байтах), доступный для хранения резервных копий с заданным `file_prefix` в директории `path_prefix`. При недостатке места создание резервной копии завершится с ошибкой. Значение `0` отключает проверку занимаемого места на диске. |

#### Дополнительные параметры для облачного хранилища

| Параметр | Тип | Обязательный | Значение по умолчанию | Описание |
|----------|-----|--------------|-----------------------|----------|
| `aws_s3_bucket` | Строка | Да | — | Имя S3 bucket для хранения резервных копий. |
| `aws_s3_region` | Строка | Нет | — | Регион S3 bucket. |
| `aws_access_key_id` | Строка | Нет | — | Идентификатор ключа для доступа к S3 bucket. |
| `aws_secret_access_key` | Строка | Нет | — | Секретный ключ для доступа к S3 bucket. |
| `aws_s3_endpoint` | Строка | Нет | — | Эндпойнт S3-сервиса. |
| `aws_s3_disable_tls` | Булевый | Нет | — | Отключает TLS для S3-эндпойнта. Используется только для тестирования, обычно в сочетании с `aws_s3_endpoint`. |
| `aws_s3_ca_certificate` | Строка | Нет | — | Сертификат CA для S3-эндпойнта в формате PEM. |

### Примеры запросов

#### Создание конфигурации

Указываются все обязательные поля.

```shell
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "5m",
    "path_prefix":       "backups",
    "file_prefix":       "main_stronghold",
    "retain":            "4",
    "storage_type":      "aws-s3",
    "aws_s3_bucket":         "my_bucket",
    "aws_s3_endpoint":       "minio.domain.ru",
    "aws_access_key_id":     "oWdPcQ50zTuMjJI",
    "aws_secret_access_key": "4NzZjboafWyfNTe7aUVgLUdrMurHjty43iUXHFBw"
}
EOF
```

Пример ответа:

```console
Key    Value
---    -----
msg    successfully created config
```

#### Обновление конфигурации

Допускается указывать не все поля. Уже существующие поля не будут изменены.

```shell
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "3m",
    "retain":            "10",
    "aws_access_key_id":     "vnR9Rfp0toPPgK3",
    "aws_secret_access_key": "FuloGN1RZCtwINCLJtwHXTQ50zCL7s"
}
EOF
```

Пример ответа:

```console
Key    Value
---    -----
msg    successfully updated config
```

## Просмотр списка существующих конфигураций

| Метод | Путь |
|-------|------|
| LIST  | `/sys/storage/raft/snapshot-auto/config` |

Возвращает список всех существующих конфигураций автоматического резервного копирования.

### Пример запроса

```shell
d8 stronghold list sys/storage/raft/snapshot-auto/config
```

Пример ответа:

```console
Keys
----
s3every5min
localEvery3min
```

## Получение параметров конфигурации

| Метод | Путь |
|-------|------|
|  GET  | `/sys/storage/raft/snapshot-auto/config/:name` |

Возвращает значения всех параметров указанной конфигурации.

### Пример запроса

```shell
d8 stronghold read sys/storage/raft/snapshot-auto/config/s3every5min
```

Пример ответа:

```console
Key                     Value
---                     -----
interval                300
path_prefix             backups
file_prefix             main_stronghold
retain                  4
storage_type            aws-s3
aws_s3_bucket           my_bucket
aws_s3_disable_tls      false
aws_s3_endpoint         minio.domain.ru
aws_s3_region           n/a
aws_s3_ca_certificate   n/a
```

## Удаление конфигурации

| Метод  | Путь |
|--------|------|
| DELETE | `/sys/storage/raft/snapshot-auto/config/:name` |

Удаляет указанную конфигурацию и возвращает информацию о последней созданной резервной копии.

### Пример запроса

```shell
d8 stronghold delete sys/storage/raft/snapshot-auto/config/s3every5min
```

Пример ответа:

```console
Key                    Value
---                    -----
consecutive_errors     0
last_snapshot_end      2025-01-31T15:24:14Z
last_snapshot_error    n/a
last_snapshot_start    2025-01-31T15:24:12Z
last_snapshot_url      https://minio.domain.ru/my_bucket/backups/main_stronghold_2025-01-31T15:24:12Z
next_snapshot_start    2025-01-31T15:29:12Z
snapshot_start         2025-01-31T15:24:12Z
snapshot_url           https://minio.domain.ru/my_bucket/backups/main_stronghold_2025-01-31T15:24:12Z
```

## Получение статуса резервной копии

| Метод | Путь |
|-------|------|
|  GET  | `/sys/storage/raft/snapshot-auto/status/:name` |

Возвращает информацию о текущем статусе указанной резервной копии.

### Пример запроса

```shell
d8 stronghold read sys/storage/raft/snapshot-auto/status/s3every5min
```

Пример ответа:

```console
Key    Value
---    -----
msg    successfully deleted config
```
