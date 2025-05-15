---
title: "Обслуживание"
permalink: ru/code/documentation/admin/configuration/maintenance.html
lang: ru
---

## Toolbox

Включенные в Toolbox инструменты используются для обслуживания и выполнения административных задач над продуктом Code.

Toolbox содержит множество полезных GitLab-инструментов, таких как `rails-console`, `rake-tasks`, `backup-utillity` и т.д. Они позволяют совершать такие задачи, как просмотр статуса миграции в БД, профилирование rails-приложений, запуск `rake-tasks` в административных целях, взаимодействовать с `rails-console` или выполнять процедуру восстановления из бэкапов:

```shell
# locate the Toolbox pod
kubectl -n d8-code get pods -lapp.kubernetes.io/component=toolbox

# Launch a shell inside the pod
kubectl exec -it <Toolbox pod name> -- bash

# open Rails console
gitlab-rails console -e production

# execute a Rake task
gitlab-rake gitlab:env:info
```

### Советы по rails-консоли

`Rails-console` является одним из компонентов Toolbox.

Через rails-консоль можно выполнить множество полезных и иногда экстренных задач. Доступ к консоли предоставляет административные права по умолчанию, поэтому настоятельно рекомендуется использовать её с осторожностью и только в случае крайней необходимости.

### Как отключить пайплайны для всех проектов

Для отключения пайплайнов для всех проектов выполните команду:

```ruby
Project.all.find_each { |project| project.update!(builds_enabled: false) }
```

### Как включить стандартную аутентификацию по паролю

Для включения стандартной аутентификации по паролю выполните команду:

```ruby
Gitlab::CurrentSettings.update!(password_authentication_enabled_for_web: true)
```

## Резервное копирование и восстановление

Скрипт резервного копирования создает архивный файл для сохранения ваших данных.

Скрипт выполняет следующие действия:

- Извлекает предыдущий архивный файл резервной копии, если вы выполняете инкрементное резервное копирование.
- Обновляет или создает новый архивный файл резервной копии.
- Выполняет все подзадачи резервного копирования:
  - Создает резервную копию базы данных.
  - Создает резервную копию Git-репозиториев.
  - Создает резервную копию файлов (включая S3-хранилища).
- Архивирует подготовленную область резервного копирования в tar-файл.
- Загружает новый архив резервной копии в объектное хранилище.
- Очищает файлы подготовленной области резервного копирования, которые были заархивированы.

### Резервное копирование по расписанию

Убедитесь, что резервное копирование включено в спецификации `ModuleConfig`. Пример конфигурации секции описан [в документации](../../reference/examples.html#настройка-резервного-копирования).

Резервное копирование (бэкап) реализовано с помощью Kubernetes cronJobs (расписание cron-а также может быть настроено). Используется нативный инструмент GitLab- `backup-utlity`, а сам процесс подробно описан в [официальной документации](https://docs.gitlab.com/charts/backup-restore/backup.html). Стоит отметить, что резервное копирование производится с использованием ключа `--repositories-server-side`. что обеспечивает асинхронное снятие копий непосредственно с реплик Gitaly. [Подробнее](https://docs.gitlab.com/ee/administration/backup_restore/backup_archive_process.html#server-side-backups).

### Особенности конфигурации диска

> Размер диска вычисляется по формуле `Gitaly node size` + `sum of all buckets size` + `database size`.

- Убедитесь, что выделенного размера диска достаточно для хранения файла резервной копии.
- Альтернативным вариантом будет отключение `persistenceVolume` для бэкапов (`backup.persistentVolumeClaim.enabled: false`). В таком случае, следует убедиться, что на узле Kubernetes, где будут запущены `backup-cronjob` и `toolbox` (для восстановления из бэкапа) достаточно места на дисках.

Для включения регулярного резервного копирования добавьте следующую секцию в `ModuleConfig`:

```yaml
backup:
  cronSchedule: 0 0 */7 * *
  enabled: true
  s3:
    bucketName: d8-code-test-backups
    external:
      provider: YCloud
      accessKey: __ACCESS_KEY__
      secretKey: __SECRET_KEY__
    mode: External
  persistentVolumeClaim:
    enabled: true # использовать ли persistentStorage при бэкапе и восстановлении
    storageClass: localpath
```

После правильной настройки `ModuleConfig` нет необходимости в дополнительных шагах. Просто дождитесь, пока бэкап выполнится по расписанию.

### Резервное копирование по требованию

1. Убедитесь, что секция `backup.s3` задана в `moduleConfig`.
1. Убедитесь, что необходимый компонент запущен и готов к работе:

   ```shell
   kubectl -n d8-code get pods -lapp.kubernetes.io/component=toolbox
   ```

1. Запустите утилиту резервного копирования:

   ```shell
   kubectl exec -n d8-code deploy/toolbox -it -- backup-utility
   ```

   Резервная копия будет сохранена в бакете `backup.s3.bucketName`. Его имя будет соответствовать <timestamp>_`gitlab_backup.tar` формату.

### Восстановление из резервной копии

Для восстановления из резервной копии следуйте нижеприведенной инструкции:

1. Перейдите в `restore mode` с помощью переключения `backup.restoreFromBackupMode` в значение `true` в `ModuleConfig`. Это позволит выключить все компоненты-потребители Code на время процесса восстановления.
1. Используйте toolbox-под и встроенную в него утилиту `backup-utlity` для старта процесса восстановления: `kubectl -n d8-code exec <Toolbox pod name> -it -- backup-utility --restore -t <timestamp|URL>`:
   - `timestamp` - дата из имени целевого архива резервной копии.
   - `URL` - публичный адрес файла резервной копии, удовлетворяющий `file:///path` формату.
1. Следуйте процессу интерактивного режима восстановления: отвечайте `yes` на все предложения утилиты.
1. Как только Toolbox завершит восстановление, верните все временно удаленные компоненты с помощью выключения `restore mode` флагом `backup.restoreFromBackupMode: false` в `ModuleConfig`.

> Вы можете дополнительно верифицировать целостность выгружаемых данных из того же пода Toolbox, следуя инструкции из [официальной документации](https://docs.gitlab.com/ee/administration/raketasks/check.html#uploaded-files-integrity).
