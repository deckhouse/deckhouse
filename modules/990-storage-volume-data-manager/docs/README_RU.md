---
title: "Модуль storage-volume-data-manager"
description: "Модуль storage-volume-data-manager: общие концепции и положения."
moduleStatus: preview
---

Модуль storage-volume-data-manager обеспечивает механизм экспорта содержимого пользовательского тома по протоколу HTTP.

Создает namepsaced-ресурс "DataExport" в том namespace, в котором нужно создать экспорт данных.
В этом ресурсе указывается targetRef - ссылка на ресурс, который нужно экспортировать.
Поддерживаются только PersistentVolumeClaim и VolumeSnapshot.

За основу взят стандартный файловый сервер Go. Поддерживается экспорт томов в режиме файловой системы и в блочном режиме.
Обеспечены авторизации пользователя средствами k8s, механизм скачивания файлов/блочки в диапазонах байт (поддерживаются заголовки 'Range').

## Ключевые параметры

- ttl - это время после последнего обращения к серверу: скачивания файла или листинга директории. По истечении ttl экспортер-под удаляется, пользовательская PVC возвращается к пользовательскому PV. 
 В ресурсе DataExport в Condition Ready устанавливается статус false, Reason - в Expired

- publish - значение true в publish означает, что к экспортер-поду будет открыт доступ извне кластера.
При этом в поле PublicURL появится строка для публичного доступа вида: publicURL: `https://data-exporter.<public-domain>/<namespace>/<user-pvc-name>/`

## Быстрый старт

Включение модуля:

```bash
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: storage-volume-data-manager
spec:
  enabled: true
  version: 1
EOF
```

Для создания и работы с ресурсами DataExport используется команда d8, структура выглядит следующим образом:

```bash
d8 data -n <namespace> create <DataExport resource name> <тип ресурса для экспорта>/<имя ресурса для экспорта> --ttl 5m  --publish (true/false)
```

Важно!
Работа с PVC ресурсами вожможна, если PVC не используется подами в данный момент

Для примера, создание ресурса DataExport для PVC с именем "data" в namespace "project" c ttl 5m:

```bash
d8 data -n project create my-export pvc/data --ttl 5m
```

Получить информацию о созданном ресурсе можно командой:

```bash
d8 k -n project get de my-export
```

Скачивание данных производится следующей командой:

```bash
d8 data -n <namespace> download <тип ресурса (pvc/vs/dataexport)>/<имя ресурса>/<путь к файлу> -o <имя файла>
```

Например:

```bash
d8 data -n project download dataexport/my-export -o test_file.txt --publish true
```
