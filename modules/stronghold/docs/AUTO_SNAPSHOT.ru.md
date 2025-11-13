---
title: "Руководство администратора модуля stronghold по API автоматического бэкапа"
LinkTitle: "Руководство администратора по API автоматического бэкапа"
description: "Руководство администратора по работе с API автоматического бэкапа модуля stronghold."
---

Модуль stronghold поддерживает создание расписания для выполнения автоматических резервных копий хранилища секретов.
Так как Stronghold хранит данные на диске в зашифрованном виде, то резервная копия тоже будет содержать только зашифрованные данные.
Для получения доступа к данным нужно будет развернуть резервную копию в кластер Stronghold и провести процедуру распечатывания хранилища.

Поддерживается сохранение резервных копий на локальный диск в выбранную папку, или в S3-совместимое хранилище.

Управление настройками резервных копий и просмотр их статуса доступны через API, CLI и UI.

## Создание/обновление конфигурации автоматического бэкапа

| Метод | Путь |
|-------|------|
| POST  | /sys/storage/raft/snapshot-auto/config/:name |

Для взаимодействия с данным методом API потребуются sudo права

### Параметры

- **interval (integer or string: <required>)** - Время между снэпшотами. Это может быть либо целое число секунд, либо строка формата Go duration (например, 24h).

- **retain (integer: 3)** - Сколько снимков должно храниться; при записи снимка, если уже хранится больше снимков, чем это число, самые старые будут удалены.

- **path_prefix (immutable string: <required>)** - Для `storage_type = local` - каталог, в который будут записываться снимки. Для типов облачных хранилищ - префикс bucket, который следует использовать, также игнорируется ведущее \`/\`. Последующие \`/\` необязательны. Неизменяемое значение.

- **file_prefix (immutable string: "stronghold-snapshot")** - В пределах каталога или префикса бакета, заданного \`path_prefix\`, имя файла или объекта снэпшота будет начинаться с этой строки. Неизменяемое значение.

- **storage_type (immutable string: <required>)** - Одно из значений `local` или `aws-s3`. Остальные параметры, описанные ниже, специфичны для выбранного `storage_type` и имеют соответствующий префикс. Неизменяемое значение.

#### storage_type = "local"
- **local_max_space (integer: 0)** - Для `storage_type=local` максимальное пространство в байтах, которое будет использоваться для всех снимков с заданным `file_prefix` в каталоге `path_prefix`. Попытки создания снэпшотов будут неудачными, если будет недостаточно места. Значение 0 (по умолчанию) отключает проверку занимаемого места.

#### storage_type = "aws-s3"
- **aws_s3_bucket (string: <required>)** - Название S3 бакета для записи снэпшотов.
- **aws_s3_region (string)** - Регион S3 бакета.
- **aws_access_key_id (string)** - Идентификатор ключа для доступа в S3 бакет.
- **aws_secret_access_key (string)** - Секретный ключ для доступа в S3 бакет.
- **aws_s3_endpoint (string)** - Адрес сервиса S3 бакета.
- **aws_s3_disable_tls (boolean)** - Отключение TLS для конечной точки S3. Этот параметр следует использовать только для тестирования, обычно в сочетании с `aws_s3_endpoint`.
- **aws_s3_ca_certificate (string)** - Сертификат центра сертификации для конечной точки в формате PEM.

### Пример
#### Создание

Указываются все обязательные поля
```sh
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "5m",
    "path_prefix":       "backups",
    "file_prefix":       "main_stronghold",
    "retain":            "4",
    "storage_type":      "aws-s3",
    "aws_s3_bucket":         "my_bucket",
    "aws_s3_endpoint":       "minio.domain.ru",
    "aws_access_key_id":     "<AWS_ACCESS_KEY_ID>",
    "aws_secret_access_key": "<AWS_SECRET_ACCESS_KEY>"
}
EOF
```

Ответ:
```
Key    Value
---    -----
msg    successfully created config
```

#### Обновление

Можно указывать не все поля, уже существующие поля не будут изменены
```sh
d8 stronghold write sys/storage/raft/snapshot-auto/config/s3every5min - <<EOF
{
    "interval":          "3m",
    "retain":            "10",
    "aws_access_key_id":     "<AWS_ACCESS_KEY_ID>",
    "aws_secret_access_key": "<AWS_SECRET_ACCESS_KEY>"
}
EOF
```

Ответ:
```
Key    Value
---    -----
msg    successfully updated config
```

## Вывод списка существующих конфигураций автоматического бэкапа

| Метод | Путь |
|-------|------|
| LIST  | /sys/storage/raft/snapshot-auto/config |

Используется для получения списка названий всех существующих автоматических снэпшотов
### Пример

`d8 stronghold list sys/storage/raft/snapshot-auto/config`

Ответ:
```
Keys
----
s3every5min
localEvery3min
```

## Получение параметров конфигурации автоматического бэкапа

| Метод | Путь |
|-------|------|
|  GET  | /sys/storage/raft/snapshot-auto/config/:name |

### Пример

`d8 stronghold read sys/storage/raft/snapshot-auto/config/s3every5min`

Ответ:
```
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

## Удаление конфигурации автоматического бэкапа

| Метод  | Путь |
|--------|------|
| DELETE | /sys/storage/raft/snapshot-auto/config/:name |

### Пример

`d8 stronghold delete sys/storage/raft/snapshot-auto/config/s3every5min`

Ответ:
```
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

## Получение статуса работы автоматического бэкапа

| Метод | Путь |
|-------|------|
|  GET  | /sys/storage/raft/snapshot-auto/status/:name |

### Пример

`d8 stronghold read sys/storage/raft/snapshot-auto/status/s3every5min`

Ответ:
```
Key    Value
---    -----
msg    successfully deleted config
```
