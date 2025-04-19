---
title: "Примеры"
permalink: ru/code/documentation/reference/examples.html
lang: ru
---

## Pages

Если шаблон публичных доменов — `%s.example.com`, то `Pages` будут доступны по адресу `https://code-pages.example.com` (поддомен `code`).

## Generic

Пример использования `Generic` S3 (S3-compatible сервис) для развертывания:

```yaml
features:
  pages:
    enabled: true
    objectStorage:
      mode: External
      bucketPrefix: d8
      external:
        provider: Generic
        region: <REPLACE_ME>
        accessKey: <REPLACE_ME>
        secretKey: <REPLACE_ME>
        endpoint: <REPLACE_ME> # например "http://minio.example.com:9090"
```

## YCloud

Пример использования YCloud S3 для развертывания:

```yaml
features:
  pages:
    enabled: true
    objectStorage:
      mode: External
      bucketPrefix: d8
      external:
        provider: YCloud
        accessKey: <REPLACE_ME>
        secretKey: <REPLACE_ME>
```

[Пример использования](#examples.html#манифест-для-описанной-конфигурации-s3) стандартной конфигурации YCloud S3 c выключенным серверным шифрованием.

## AzureRM

Пример использования AzureRM для развертывания:

```yaml
features:
  pages:
    enabled: true
    objectStorage:
      mode: External
      bucketPrefix: d8
      external:
        provider: AzureRM
        azureAccountName: <REPLACE_ME>
        azureAccessKey: <REPLACE_ME>
```

Пример использования AWS c SSE с включенным серверным шифрованием (шифрование на стороне сервера):

```yaml
objectStorage:
  mode: External
  bucketPrefix: d8
  external:
    provider: AWS
    region: <REPLACE_ME>
    accessKey: <REPLACE_ME>
    secretKey: <REPLACE_ME>
    storage_options:
      server_side_encryption: aws:kms
      server_side_encryption_kms_key_id: <REPLACE_ME> # например arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab
```

## Внешний Redis

Примеры различной конфигурации внешнего Redis-кластера:

1. Один сервер:

   ```yaml
   redis:
     external:
       auth:
         enabled: true
         password: <REPLACE_ME>
       host: 10.10.10.2
       port: 6379
     mode: External
   ```

1. С использованием sentinel:

   ```yaml
   redis:
     external:
       auth:
         enabled: true
         password: <REPLACE_ME>
       masterName: <REPLACE_ME>
       port: 6379
       sentinels:
         - host: <REPLACE_ME> # sentinel host #1
           port: 26379
         - host: <REPLACE_ME> # sentinel host #2
     mode: External
   ```

### Настройка Managed Service for Redis

Для создания кластера Managed Service for Redis необходимы:

- роль [vpc.user](https://yandex.cloud/ru/docs/vpc/security/#vpc-user);
- роль [managed-redis.editor или выше](https://yandex.cloud/ru/docs/managed-postgresql/security/#roles-list).

В [консоли управления](https://console.yandex.cloud/) выберите каталог, в котором нужно создать кластер БД.

1. Выберите сервис «Managed Service for Redis».
1. Нажмите кнопку «Создать кластер».
1. В блоке «Базовые параметры»:
   - Введите имя кластера в поле «Имя кластера». Имя кластера должно быть уникальным в рамках каталога.
   - (Опционально) Добавьте описание кластера.
   - Выберите окружение, в котором нужно создать кластер (после создания кластера окружение изменить невозможно):
     - `PRODUCTION` — для стабильных версий ваших приложений.
     - `PRESTABLE` — для тестирования. Prestable-окружение аналогично Production-окружению и на него также распространяется SLA, но при этом на нем раньше появляются новые функциональные возможности, улучшения и исправления ошибок. В Prestable-окружении вы можете протестировать совместимость новых версий с вашим приложением.
1. Выберите версию СУБД (рекомендовано 7.0+).
1. Нажмите кнопку «Создать кластер».
1. Создание кластера займет некоторое время.

### Манифест для описанной конфигурации Redis

```yaml
redis:
  mode: External
  external:
    host: <REPLACE_ME> # FQDN of master хоста
    port: 6379
    auth:
      enabled: true
      password: <REPLACE_ME>
```

## Внешний PostgreSQL

1. Пример без TLS:

   ```yaml
   postgres:
     external:
       database: db
       host: <REPLACE_ME> # адрес мастер-хоста
       port: 5432
       username: <REPLACE_ME>
       password: <REPLACE_ME>
       praefectDatabase: praefect
       praefectUsername: <REPLACE_ME>
       praefectPassword: <REPLACE_ME>
     mode: External
   ```

1. Пример с включенным TLS:

   ```yaml
   postgres:
     external:
       database: db
       host: <REPLACE_ME> # адрес мастер-хоста
       port: 5432
       username: simple_user
       sslmode: verify-full
       serverCA: |
         # postgres server CA
       clientCert: |
         # Your TLS certificate
       clientKey: |
       # Your TLS key
     mode: External
   ```

   Вы также можете использовать TLS авторизацию при подключении к основной базе данных.

### Настройка Managed Service for PostgreSQL

Для создания кластера Managed Service for PostgreSQL необходимы:

- роль [vpc.user](https://yandex.cloud/ru/docs/vpc/security/#vpc-user) и
- роль [managed-postgresql.editor или выше](https://yandex.cloud/ru/docs/managed-postgresql/security/#roles-list).

В [консоли управления](https://console.yandex.cloud/) выберите каталог, в котором нужно создать кластер БД:

1. Выберите сервис «Managed Service for PostgreSQL».
1. Нажмите кнопку «Create cluster».
1. Введите имя кластера в поле «Имя кластера». Имя кластера должно быть уникальным в рамках каталога.
1. Выберите окружение, в котором нужно создать кластер (после создания кластера окружение изменить невозможно):
   - `PRODUCTION` — для стабильных версий ваших приложений.
   - `PRESTABLE` — для тестирования. Prestable-окружение аналогично Production-окружению и на него также распространяется
  SLA, но при этом на нем раньше появляются новые функциональные возможности, улучшения и исправления ошибок. В
  Prestable-окружении вы можете протестировать совместимость новых версий с вашим приложением.
1. Выберите версию СУБД (рекомендована 16+).
1. Выберите класс хостов — он определяет технические характеристики виртуальных машин, на которых будут развернуты хосты
   БД. Все доступные варианты перечислены в разделе «Классы хостов». При изменении класса хостов для кластера меняются
   характеристики всех уже созданных хостов.

1. В блоке База данных укажите атрибуты БД:

   - Имя БД. Это имя должно быть уникальным в рамках каталога.
     Имя базы может содержать латинские буквы, цифры, подчеркивание и дефис. Максимальная длина имени 63 символа. Имена
     `postgres`, `template0`, `template1` зарезервированы для собственных нужд Managed Service for PostgreSQL. Создавать
     базы с этими именами нельзя.
   - Имя пользователя — владельца БД и пароль. По умолчанию новому пользователю выделяется 50 подключений к каждому хосту
     кластера.
   - Локаль сортировки и локаль набора символов. Эти настройки определяют правила, по которым производится сортировка
     строк (`LC_COLLATE`) и классификация символов (`LC_CTYPE`). В Managed Service for PostgreSQL настройки локали
     действуют на уровне отдельно взятой БД. По умолчанию используется локаль `C`. Подробнее о настройках локали в
     [документации PostgreSQL](https://www.postgresql.org/docs/current/locale.html).

1. Нажмите кнопку «Создать кластер».
1. Создание кластера займет некоторое время.
1. Выберите кластер из списка и перейдите на вкладку «Базы данных». Выберите базу данных и включите следующие расширения:
1. Добавьте базу данных для компонента Praefect:
1. Перейдите на вкладку «Пользователи» и выставьте лимит подключений. Мы рекомендуем выставить лимит не менее 150.

### Манифест для описанной конфигурации PostgreSQL

```yaml
postgres:
  mode: External
  external:
    host: <REPLACE_ME> # FQDN-адрес мастер хоста
    port: 6432
    database: <REPLACE_ME>
    username: <REPLACE_ME>
    password: <REPLACE_ME>
    praefectDatabase: praefect
    praefectUsername: <REPLACE_ME> # идентично значению postgres.username
    praefectPassword: <REPLACE_ME> # идентично значению postgres.password
```

## Настройка резервного копирования

Пример настройки:

```yaml
backup:
  enabled: true
  cronSchedule: "0 0 1 * *"
  s3:
    bucketName: <REPLACE_ME>
    tmpBucketName: <REPLACE_ME>
    mode: External
    external:
      accessKey: <REPLACE_ME>
      provider: <REPLACE_ME>
      region: <REPLACE_ME>
      secretKey: <REPLACE_ME>
  persistentVolumeClaim:
    enabled: <true|false>
    storageClass: network-hdd
```

## Настройка S3

Создайте пользователя с именем `d8-code-sa`. В ответ вернутся параметры пользователя:

```shell
yc iam service-account create --name d8-code-sa
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: d8-code-sa
```

Назначьте роль `storage.editor` вновь созданному пользователю для своего облака:

```shell
yc resource-manager folder add-access-binding <folderID> --role storage.editor --subject serviceAccount:<userID>
```

Создайте `Access key` для пользователя. В дальнейшем с помощью этих данных будем авторизовываться в облаке:

```shell
yc iam access-key create --service-account-name  d8-code-sa
access_key:
  id: <id>
  service_account_id: <userID>
  created_at: "YYYY-MM-DDTHH:MM:SSZ"
  key_id: <ACCESS_KEY>
secret: <SECRET_KEY>
```

### Манифест для описанной конфигурации S3

```yaml
features:
  pages:
    enabled: true
    objectStorage:
      mode: External
      bucketPrefix: <REPLACE_ME>
      external:
        accessKey: <REPLACE_ME> # accesskey.key_id полученный на предыдущем шаге
        provider: YCloud
        secretKey: <REPLACE_ME> # secretkey полученный на предыдущем шаге
...
objectStorage:
  mode: External
  bucketPrefix: <REPLACE_ME>
  external:
    provider: YCloud
    accessKey: <REPLACE_ME> # accesskey.key_id полученный на предыдущем шаге
    secretKey: <REPLACE_ME> # secretkey полученный на предыдущем шаге
    proxy_download: true
```
