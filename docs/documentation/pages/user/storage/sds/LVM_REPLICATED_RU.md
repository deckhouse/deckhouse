---
title: "Использование реплицируемого хранилища на основе DRBD"
permalink: ru/storage/user/sds/lvm-replicated.html
lang: ru
---

{% alert level="warning" %}
Работоспособность гарантируется только при соблюдении [системных требований](#системные-требования). Использование в иных условиях возможно, однако стабильная работа не гарантируется.
{% endalert %}

## Системные требования

{% alert level="info" %}
Применительно как к однозональным кластерам, так и к кластерам с использованием нескольких зон доступности.
{% endalert %}

- Используйте стоковые ядра, поставляемые вместе с поддерживаемыми дистрибутивами.
- Для сетевого соединения необходимо использовать инфраструктуру с пропускной способностью 10 Gbps или выше.
- Чтобы достичь максимальной производительности, сетевая задержка между узлами должна находиться в пределах 0,5–1 мс.
- Не используйте другой SDS (Software defined storage) для предоставления дисков SDS Deckhouse.

## Рекомендации

- Не используйте RAID. Подробнее [ниже](#причины-отказа-от-raid-с-sds-replicated-volume).
- Используйте локальные физические диски. Подробнее [ниже](#рекомендации-по-использованию-локальных-дисков).
- Для стабильной работы кластера, но с ухудшением производительности, допустимая сетевая задержка между узлами не должна превышать 20 мс.

## Получение информации об использовании пространства

Доступно два способа:

1. Через дашборд Grafana:

   Перейдите в раздел «Dashboards» → «Storage» → «LINSTOR/DRBD». В правом верхнем углу отображается текущий уровень занятости пространства кластера.

   > **Внимание.** Значение отражает состояние всего доступного пространства. При создании томов с двумя репликами полученные цифры следует делить на два, чтобы оценить, сколько томов реально можно разместить.

1. Через командный интерфейс:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor storage-pool list
   ```

   > **Внимание.** Значение отражает состояние всего доступного пространства. При создании томов с двумя репликами полученные цифры следует делить на два, чтобы оценить, сколько томов реально можно разместить.

## Назначение StorageClass по умолчанию

Для назначения StorageClass по умолчанию необходимо в пользовательском ресурсе [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/) в поле `spec.isDefault` указать значение `true`. 

## Добавление существующей LVMVolumeGroup или LVMThin‑пула

1. Присвойте Volume Group LVM-тег `storage.deckhouse.io/enabled=true`:

   ```shell
   vgchange myvg-0 --add-tag storage.deckhouse.io/enabled=true
   ```

   После этого Volume Group будет автоматически обнаружена, и для неё будет создан ресурс [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/).

1. Полученный ресурс укажите в параметрах [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) в поле `spec.lvmVolumeGroups[].name` Если используется LVMThin‑пул, дополнительно задайте имя в `spec.lvmVolumeGroups[].thinPoolName`.

## Изменение ограничений DRBD‑томов и портов кластера

Портовой диапазон по умолчанию для DRBD-ресурсов — TCP 7000–7999. Переопределить его можно с помощью настройки `drbdPortRange`, указав нужные значения `minPort` и `maxPort`.

{% alert level="warning" %}
После изменения параметров `drbdPortRange` перезапустите контроллер LINSTOR, чтобы новые настройки вступили в силу. При этом существующие DRBD-ресурсы сохранят назначенные им порты.
{% endalert %}

## Корректная перезагрузка узла с DRBD‑ресурсами

{% alert level="info" %}
Чтобы обеспечить стабильную работу модуля, избегайте перезагрузки нескольких узлов одновременно:
{% endalert %}

1. Выполните drain нужного узла:

   ```shell
   d8 k drain test-node-1 --ignore-daemonsets --delete-emptydir-data
   ```

1. Проверьте отсутствие проблемных DRBD-ресурсов или ресурсов в состоянии `SyncTarget`. Если такие обнаружены — дождитесь завершения синхронизации или выполните корректирующие действия:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -t deploy/linstor-controller -- linstor r l --faulty
   ```

   Пример вывода:

   ```console
   Defaulted container "linstor-controller" out of: linstor-controller, kube-rbac-proxy
   +----------------------------------------------------------------+
   | ResourceName | Node | Port | Usage | Conns | State | CreatedOn |
   |================================================================|
   +----------------------------------------------------------------+
   ```

1. Перезагрузите узел и дождитесь синхронизации всех DRBD-ресурсов, затем выполните `uncordon`: 

   ```shell
   d8 k uncordon test-node-1
   node/test-node-1 uncordoned
   ```

При необходимости перезагрузки еще одного узла — повторите алгоритм.

## Перемещение ресурсов для освобождения места в Storage Pool

1. Просмотрите список Storage Pool:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor storage-pool list -n OLD_NODE
   ```

1. Определите расположение томов:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor volume list -n OLD_NODE
   ```

1. Получите список ресурсов для переноса:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor resource list-volumes
   ```

1. Переместите выбранные ресурсы на другой узел (не более 1–2 одновременно):

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing resource create NEW_NODE RESOURCE_NAME
   ```

1. Дождитесь синхронизации:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor resource-definition wait-sync RESOURCE_NAME
   ```

1. Удалите ресурс с исходного узла:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing resource delete OLD_NODE RESOURCE_NAME
   ```

## Вытеснение DRBD‑ресурсов с узла

Скрипт `evict.sh` выполняет вытеснение в двух режимах:

- Удаление узла — создаются дополнительные реплики для каждого ресурса, после чего узел удаляется из LINSTOR и Kubernetes.

- Удаление ресурсов — создаются реплики для ресурсов, затем сами ресурсы удаляются из LINSTOR (узел остаётся в кластере).

### Подготовка и запуск скрипта

Перед выполнением вытеснения:

1. Убедитесь в наличии скрипта на master-узле:

   ```shell
   ls -l /opt/deckhouse/sbin/evict.sh
   ```

2. Исправьте ошибочные ресурсы:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor resource list --faulty
   ```

3. Проверьте, что все поды в пространстве имён `d8-sds-replicated-volume` находятся в состоянии `Running`:

   ```shell
   d8 k -n d8-sds-replicated-volume get pods | grep -v Running
   ```

### Пример удаления узла из LINSTOR и Kubernetes

Запустите скрипт `evict.sh` на любом из master-узлов в интерактивном режиме, указав режим удаления `--delete-node`:

```shell
/opt/deckhouse/sbin/evict.sh --delete-node
```

Для неинтерактивного режима добавьте флаг `--non-interactive` и укажите имя узла. В этом режиме скрипт выполнит все действия без запроса подтверждения от пользователя:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-node --node-name "worker-1"
```

### Пример удаления ресурсов с узла

Запустите скрипт `evict.sh` на любом из master-узлов в интерактивном режиме, указав режим удаления `--delete-resources-only`:

```shell
/opt/deckhouse/sbin/evict.sh --delete-resources-only
```

Для неинтерактивного режима добавьте флаг `--non-interactive` и укажите имя узла. В этом режиме скрипт выполнит все действия без запроса подтверждения от пользователя:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-resources-only --node-name "worker-1"
```

{% alert level="warning" %}
По окончании работы скрипта узел остаётся в статусе `SchedulingDisabled`, а в LINSTOR выставляется свойство `AutoplaceTarget=false`. Это блокирует автоматическое создание новых ресурсов на узле.
{% endalert %}

Чтобы вновь разрешить размещение ресурсов, выполните:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node set-property "worker-1" AutoplaceTarget
kubectl uncordon "worker-1"
```

Проверьте параметр `AutoplaceTarget` у всех узлов (поле `AutoplaceTarget` будет пустым у тех узлов, на которых разрешено размещать ресурсы LINSTOR):

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node list -s AutoplaceTarget
```

### Параметры скрипта `evict.sh`

- `--delete-node` — удаление узла из LINSTOR и Kubernetes с предварительным созданием дополнительных реплик для всех ресурсов, размещенных на узле;
- `--delete-resources-only` — удаление ресурсов с узла без удаления узла из LINSTOR и Kubernetes с предварительным созданием дополнительных реплик для всех ресурсов, размещенных на узле;
- `--non-interactive` — запуск скрипта в неинтерактивном режиме;
- `--node-name` — имя узла, с которого необходимо вытеснить ресурсы. Параметр обязателен для использования в режиме `--non-interactive`;
- `--skip-db-backup` — пропустить создание резервной копии БД LINSTOR перед выполнением операций;
- `--ignore-advise` — выполнить операции, несмотря на предупреждения команды `linstor advise resource`;
- `--exclude-resources-from-check` — исключить из проверки ресурсы, перечисленные через символ `|`;

## Диагностика проблем

Проблемы могут возникнуть на разных уровнях работы компонентов. Ниже приведена шпаргалка для диагностики неисправностей томов в LINSTOR.

![шпаргалка](./images/linstor-debug-cheatsheet.ru.svg)
<!--- Исходник: https://docs.google.com/drawings/d/19hn3nRj6jx4N_haJE0OydbGKgd-m8AUSr0IqfHfT6YA/edit --->

### Ошибка запуска linstor-node при загрузке DRBD‑модуля

1. Проверьте состояние подов `linstor-node`:

   ```shell
   d8 k get pod -n d8-sds-replicated-volume -l app=linstor-node
   ```

1. Если некоторые поды находятся в состоянии `Init`, проверьте версию DRBD и логи bashible на узле:

   ```shell
   cat /proc/drbd
   journalctl -fu bashible
   ```

Наиболее вероятные причины:

- Загружена версия DRBDv8 вместо требуемой DRBDv9. Проверьте версию (если файл /proc/drbd отсутствует, модуль не загружен):
  
  ```shell
  cat /proc/drbd
  ```
  
  Если файл отсутствует, значит модуль не загружен и проблема не в этом.

- Включён Secure Boot. Поскольку DRBD компилируется динамически (аналог dkms) без цифровой подписи, модуль не поддерживается при включённом Secure Boot.

### Ошибка FailedMount при запуске пода

#### При зависании пода на стадии ContainerCreating

Если под завис на стадии `ContainerCreating`, а в выводе команды `d8 k describe pod` присутствуют ошибки вроде той, что представлена ниже, значит устройство смонтировано на одном из других узлов:

```console
rpc error: code = Internal desc = NodePublishVolume failed for pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d: checking
for exclusive open failed: wrong medium type, check device health
```

Проверьте, где используется устройство, с помощью следующей команды:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor resource list -r pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d
```

Флаг `InUse` покажет, на каком узле используется устройство; на этом узле потребуется вручную отмонтировать диск.

#### Ошибки Input/output error

Такие ошибки обычно возникают на этапе создания файловой системы (mkfs). Проверьте `dmesg` на узле, где запускается под:

```shell
dmesg | grep 'Remote failed to finish a request within'
```

Если вывод содержит сообщения вида *Remote failed to finish a request within …*, возможно, дисковая подсистема недостаточно быстрая для корректной работы DRBD.

## Удаление остаточного Storage Pool после удаления ReplicatedStoragePool

Модуль `sds-replicated-volume` не обрабатывает операции удаления ресурса [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/).

## Ограничения на изменение spec ReplicatedStorageClass

Изменению подлежит только поле `isDefault`. Остальные параметры ресурса являются неизменяемыми — такое поведение является ожидаемым.

## Удаление дочернего StorageClass при удалении ReplicatedStorageClass

Если StorageClass находится в статусе `Created`, его можно удалить. При наличии иных статусов потребуется восстановление ресурса или ручное удаление StorageClass.

## Ошибки при создании Storage Pool или StorageClass

При временных внешних проблемах (например, недоступность kube-apiserver) модуль автоматически повторит выполнение неудачной операции.

## Ошибка "You're not allowed to change state of linstor cluster manually"

Операции, которые требуют ручного вмешательства, в модуле `sds-replicated-volume` частично или полностью автоматизированы. Поэтому модуль `sds-replicated-volume` ограничивает список разрешенных команд в LINSTOR. Например, автоматизировано создание Tie-Breaker, —  сам LINSTOR иногда их не создает для ресурсов с двумя репликами. Чтобы посмотреть список разрешённых команд, выполните:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor --help
```

## Восстановление БД из резервной копии

Резервные копии ресурсов бэкенда хранятся в секретах в виде YAML-файлов, сегментированных для удобства восстановления. Резервное копирование происходит автоматически по расписанию.

Пример корректно сформированной резервной копии выглядит следующим образом:

```shell
linstor-20240425074718-backup-0              Opaque                           1      28s     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
linstor-20240425074718-backup-1              Opaque                           1      28s     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
linstor-20240425074718-backup-2              Opaque                           1      28s     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
linstor-20240425074718-backup-completed      Opaque                           0      28s     <none>
```

Резервная копия хранится закодированными сегментами в секретах вида `linstor-%date_time%-backup-{0..2}`, секрет вида `linstor-%date_time%-backup-completed` не содержит данных, и служит маркером корректно отработавшего процесса резервного копирования.

### Процесс восстановления резервной копии

1. Задайте переменные окружения:

   ```shell
   NAMESPACE="d8-sds-replicated-volume"
   BACKUP_NAME="linstor_db_backup"
   ```

1. Проверьте наличие резервных копий:

   ```shell
   d8 k -n $NAMESPACE get secrets --show-labels
   ```

   Пример вывода команды:

   ```shell
   linstor-20240425072413-backup-0              Opaque                           1      33m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072413
   linstor-20240425072413-backup-1              Opaque                           1      33m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072413
   linstor-20240425072413-backup-2              Opaque                           1      33m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072413
   linstor-20240425072413-backup-completed      Opaque                           0      33m     <none>
   linstor-20240425072510-backup-0              Opaque                           1      32m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072510
   linstor-20240425072510-backup-1              Opaque                           1      32m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072510
   linstor-20240425072510-backup-2              Opaque                           1      32m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072510
   linstor-20240425072510-backup-completed      Opaque                           0      32m     <none>
   linstor-20240425072634-backup-0              Opaque                           1      31m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072634
   linstor-20240425072634-backup-1              Opaque                           1      31m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072634
   linstor-20240425072634-backup-2              Opaque                           1      31m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072634
   linstor-20240425072634-backup-completed      Opaque                           0      31m     <none>
   linstor-20240425072918-backup-0              Opaque                           1      28m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072918
   linstor-20240425072918-backup-1              Opaque                           1      28m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072918
   linstor-20240425072918-backup-2              Opaque                           1      28m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072918
   linstor-20240425072918-backup-completed      Opaque                           0      28m     <none>
   linstor-20240425074718-backup-0              Opaque                           1      10m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
   linstor-20240425074718-backup-1              Opaque                           1      10m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
   linstor-20240425074718-backup-2              Opaque                           1      10m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
   linstor-20240425074718-backup-completed      Opaque                           0      10m     <none>
   ```

1. Каждая резервная копия имеет свою метку с временем создания. Выберите нужную и скопируйте метку в переменную окружения. В качестве примера используем метку самой актуальной копии из вывода выше:

   ```shell
   LABEL_SELECTOR="sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718"
   ```

1. Создайте временный каталог для хранения частей архива:

   ```shell
   TMPDIR=$(mktemp -d)
   echo "Временный каталог: $TMPDIR"
   ```

1. Создайте пустой архив и объедините данные секретов в один файл:

   ```shell
   COMBINED="${BACKUP_NAME}_combined.tar"
   > "$COMBINED"
   ```

1. Получите список секретов по метке, дешифруйте данные и поместите данные резервной копии в архив:

   ```shell
   MOBJECTS=$(kubectl get rsmb -l "$LABEL_SELECTOR" --sort-by=.metadata.name -o jsonpath="{.items[*].metadata.name}")
   
   for MOBJECT in $MOBJECTS; do
     echo "Process: $MOBJECT"
     kubectl get rsmb "$MOBJECT" -o jsonpath="{.data}" | base64 --decode >> "$COMBINED"
   done
   ```

1. Распакуйте архив, получив файлы резервной копии:

   ```shell
   mkdir -p "./backup"
   tar -xf "$COMBINED" -C "./backup --strip-components=2
   ```

1. Проверьте содержимое резервной копии:

   ```shell
   ls ./backup
   ```

  ```shell
   TMPDIR=$(mktemp -d)
   echo "Временный каталог: $TMPDIR"
   ```

   Пример вывода:

   ```console
   ebsremotes.yaml                    layerdrbdvolumedefinitions.yaml        layerwritecachevolumes.yaml  propscontainers.yaml      satellitescapacity.yaml  secidrolemap.yaml         trackingdate.yaml
   files.yaml                         layerdrbdvolumes.yaml                  linstorremotes.yaml          resourceconnections.yaml  schedules.yaml           secobjectprotection.yaml  volumeconnections.yaml
   keyvaluestore.yaml                 layerluksvolumes.yaml                  linstorversion.yaml          resourcedefinitions.yaml  secaccesstypes.yaml      secroles.yaml             volumedefinitions.yaml
   layerbcachevolumes.yaml            layeropenflexresourcedefinitions.yaml  nodeconnections.yaml         resourcegroups.yaml       secaclmap.yaml           sectyperules.yaml         volumegroups.yaml
   layercachevolumes.yaml             layeropenflexvolumes.yaml              nodenetinterfaces.yaml       resources.yaml            secconfiguration.yaml    sectypes.yaml             volumes.yaml
   layerdrbdresourcedefinitions.yaml  layerresourceids.yaml                  nodes.yaml                   rollback.yaml             secdfltroles.yaml        spacehistory.yaml
   layerdrbdresources.yaml            layerstoragevolumes.yaml               nodestorpool.yaml            s3remotes.yaml            secidentities.yaml       storpooldefinitions.yaml
   ```

1. Восстановите необходимую сущность, применив соответствующий YAML-файл:

   ```shell
   d8 k apply -f %something%.yaml
   ```

   Либо примените bulk-apply, если нужно полное восстановление:

   ```shell
   kubectl apply -f ./backup/
   ```

## Отсутствие служебных подов sds-replicated-volume на выбранном узле

С высокой вероятностью проблемы связаны с метками на узлах.

- Проверьте [dataNodes.nodeSelector](./configuration.html#parameters-datanodes-nodeselector) в настройках модуля:

  ```shell
  d8 k get mc sds-replicated-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
  ```

- Проверьте селекторы, используемые `sds-replicated-volume-controller`:

  ```shell
  d8 k -n d8-sds-replicated-volume get secret d8-sds-replicated-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
  ```

- В секрете `d8-sds-replicated-volume-controller-config` должны быть селекторы, которые указаны в настройках модуля, а так же дополнительно селектор `kubernetes.io/os: linux`.

- Проверьте, что на нужном узле есть все указанные в секрете `d8-sds-replicated-volume-controller-config` метки:

  ```shell
  d8 k get node worker-0 --show-labels
  ```

- Если меток нет, то необходимо добавить метки через шаблоны в NodeGroup или на узел.

- Если метки есть, то необходимо проверить, есть ли на нужном узле метка `storage.deckhouse.io/sds-replicated-volume-node=`. Если метки нет, то необходимо проверить, запущен ли `sds-replicated-volume-controller` и если запущен, то проверить его логи:

  ```shell
  d8 k -n d8-sds-replicated-volume get po -l app=sds-replicated-volume-controller
  d8 k -n d8-sds-replicated-volume logs -l app=sds-replicated-volume-controller
  ```

## Дополнительная поддержка

Информация о причинах неудачных операций отображается в поле `status.reason` ресурсов [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) и [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/). При недостатке информации для диагностики рекомендуется обращаться к логам `sds-replicated-volume-controller`.

## Миграция с модуля linstor на sds-replicated-volume

При миграции временно недоступны control plane LINSTOR и его CSI, что может повлиять на операции с PV (создание, расширение или удаление).

{% alert level="warning" %}
Миграция не затрагивает пользовательские данные, поскольку происходит перенос в новое пространство имён с добавлением компонентов для управления томами.
{% endalert %}

### Порядок действий для миграции

1. Убедитесь, что в бэкенде отсутствуют неисправные ресурсы. Команда должна выводить пустой список:

   ```shell
   alias linstor='kubectl -n d8-linstor exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

   > **Внимание.** Перед миграцией все ресурсы должны быть исправны.

1. Выключите модуль `linstor`:

   ```shell
   d8 k patch moduleconfig linstor --type=merge -p '{"spec": {"enabled": false}}'
   ```

1. Дождитесь, когда пространство имён `d8-linstor` будет удалено:

   ```shell
   d8 k get namespace d8-linstor
   ```

1. Создайте ресурс ModuleConfig для `sds-node-configurator`:

   ```yaml
   d8 k apply -f -<<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-node-configurator
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь, когда модуль `sds-node-configurator` перейдет в состояние `Ready`:

   ```shell
   d8 k get moduleconfig sds-node-configurator
   ```

1. Создайте ресурс ModuleConfig для `sds-replicated-volume`:

   > **Внимание.** Если в настройках модуля `sds-replicated-volume` не будет указан параметр `settings.dataNodes.nodeSelector`, то значение для этого параметра при установке модуля `sds-replicated-volume` будет взято из модуля `linstor`. Если этот параметр не указан и там, то только в этом случае он останется пустым и все узлы кластера будут считаться узлами для хранения данных.

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-replicated-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь, когда модуль `sds-replicated-volume` перейдет в состояние `Ready`:

   ```shell
   d8 k get moduleconfig sds-replicated-volume
   ```

1. Проверьте настройки модуля `sds-replicated-volume`:

   ```shell
   d8 k get moduleconfig sds-replicated-volume -oyaml
   ```

1. Дождитесь, пока все поды в пространстве имён `d8-sds-replicated-volume` и `d8-sds-node-configurator` перейдут в состояние `Ready` или `Completed`:

   ```shell
   d8 k get po -n d8-sds-node-configurator
   d8 k get po -n d8-sds-replicated-volume
   ```

1. Измените алиас к команде `linstor` и проверьте ресурсы:

   ```shell
   alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

Если неисправные ресурсы не обнаружены, значит миграция была успешной.

### Миграция на ReplicatedStorageClass

StorageClass в данном модуле управляются через ресурс `ReplicatedStorageClass`. Вручную StorageClass создаваться не должны.

При миграции с модуля LINSTOR удалите старые StorageClass и создайте новые через ресурс `ReplicatedStorageClass` в соответствии с таблицей, представленной ниже.

Обратите внимание, что в старых StorageClass нужно смотреть опцию из секции `parameter` самого StorageClass, а указывать соответствующую опцию при создании нового необходимо в `ReplicatedStorageClass`.

| параметр StorageClass                     | ReplicatedStorageClass      | Параметр по умолчанию | Примечания                                                     |
|-------------------------------------------|-----------------------|-|----------------------------------------------------------------|
| linstor.csi.linbit.com/placementCount: "1" | replication: "None"   | | Будет создаваться одна реплика тома с данными                  |
| linstor.csi.linbit.com/placementCount: "2" | replication: "Availability" | | Будет создаваться две реплики тома с данными.                  |
| linstor.csi.linbit.com/placementCount: "3" | replication: "ConsistencyAndAvailability" | да | Будет создаваться три реплики тома с данными                   |
| linstor.csi.linbit.com/storagePool: "name" | storagePool: "name"   | | Название используемого storage pool для хранения               |
| linstor.csi.linbit.com/allowRemoteVolumeAccess: "false" | volumeAccess: "Local" | | Запрещен удаленный доступ пода к томам с данными (только локальный доступ к диску в пределах узла) |

Дополнительно можно задавать параметры:

- `reclaimPolicy` (Delete, Retain) — соответствует параметру `reclaimPolicy` у старого StorageClass.
- `zones` — перечисление зон, которые нужно использовать для размещения ресурсов (прямое указание названия зон в облаке). Обратите внимание, что удаленный доступ пода к тому с данными возможен только в пределах одной зоны.
- `volumeAccess` может принимать значения `Local` (доступ строго в пределах узла), `EventuallyLocal` (реплика данных будет синхронизироваться на узле с запущенным подом спустя некоторое время после запуска), `PreferablyLocal` (удаленный доступ пода к тому с данными разрешен, `volumeBindingMode: WaitForFirstConsumer`), `Any` (удаленный доступ пода к тому с данными разрешен, `volumeBindingMode: Immediate`).
- При необходимости использовать `volumeBindingMode: Immediate`, нужно выставлять параметр ReplicatedStorageClass volumeAccess равным `Any`.

### Миграция на ReplicatedStoragePool

Ресурс [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) позволяет создавать `Storage Pool` в бэкенде модуля. Рекомендуется создать этот ресурс даже для уже существующих `Storage Pool` и указать в этом ресурсе существующие [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/). В этом случае контроллер увидит, что соответствующие `Storage Pool` созданы, и оставит их без изменений, а в поле `status.phase` созданного ресурса будет отображено значение `Created`.

## Миграция с модуля sds-drbd на sds-replicated-volume

В процессе миграции будет недоступен управляющий слой модуля и его CSI. Это приведет к невозможности создания, расширения или удаления PV и создания или удаления подов, использующих PV DRBD на время проведения миграции.

{% alert level="warning" %}
> **Важно.** Миграция не затронет пользовательские данные, так как произойдет миграция в новоe пространство имён и будут добавлены новые компоненты, которые в будущем исполнят функциональность модуля по управлению томами.
{% endalert %}

### Порядок действий для миграции

1. Убедитесь, что в кластере отсутствуют неисправные DRBD-ресурсы. Команда должна выводить пустой список:

   ```shell
   alias linstor='kubectl -n d8-sds-drbd exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

   > **Внимание.** Перед миграцией все DRBD-ресурсы должны работать корректно.

1. Выключите модуль `sds-drbd`:

   ```shell
   d8 k patch moduleconfig sds-drbd --type=merge -p '{"spec": {"enabled": false}}'
   ```

1. Дождитесь, когда пространство имён `d8-sds-drbd` будет удалено:

   ```shell
   d8 k get namespace d8-sds-drbd
   ```

1. Создайте ресурс ModuleConfig для `sds-replicated-volume`:

   > **Внимание.** Если в настройках модуля `sds-replicated-volume` не будет указан параметр `settings.dataNodes.nodeSelector`, то значение для этого параметра при установке модуля `sds-replicated-volume` будет взято из модуля `sds-drbd`. Если этот параметр не указан и там, то только в этом случае он останется пустым и все узлы кластера будут считаться узлами для хранения данных.

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-replicated-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь, когда модуль `sds-replicated-volume` перейдет в состояние `Ready`:

   ```shell
   d8 k get moduleconfig sds-replicated-volume
   ```

1. Проверьте настройки модуля `sds-replicated-volume`:

   ```shell
   d8 k get moduleconfig sds-replicated-volume -oyaml
   ```

1. Дождитесь, пока все поды в пространстве имён `d8-sds-replicated-volume` перейдут в состояние `Ready` или `Completed`:

   ```shell
   kubectl get po -n d8-sds-replicated-volume
   ```

1. Измените алиас к команде `linstor` и проверьте ресурсы DRBD:

   ```shell
   alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

Если неисправные ресурсы не обнаружены, значит миграция была успешной.

> **Внимание.** Ресурсы DRBDStoragePool и DRBDStorageClass в процессе будут автоматически мигрированы на ReplicatedStoragePool и ReplicatedStorageClass, вмешательства пользователя для этого не требуется.

Логика работы этих ресурсов не изменится. Однако, стоит проверить, не осталось ли в кластере ресурсов DRBDStoragePool или DRBDStorageClass, если после миграции они существуют — сообщите, пожалуйста, в нашу техническую поддержку.

## Причины отказа от RAID с sds-replicated-volume

Использование DRBD с более чем одной репликой уже обеспечивает функциональность сетевого RAID. Применение RAID локально может привести к следующим проблемам:

- В несколько раз увеличивает оверхед по используемому пространству в случае использования RAID с избыточностью. Например, используется [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/) с `replication`, выставленном в `ConsistencyAndAvailability`. При таких настройках DRBD будет сохранять данные в трех репликах (по одной реплике на три разных узла). Если на этих узлах будет использоваться RAID1, то для хранения 1 Гб данных потребуется суммарно 6 Гб места на дисках. RAID с избыточностью есть смысл использовать для упрощения обслуживания серверов в том случае, когда цена хранения не имеет значения. RAID1 в таком случае позволит менять диски на серверах без необходимости перемещения реплик данных с "проблемного" диска.

- В случае RAID0 прирост производительности будет незаметен, т. к. репликация данных будет осуществляться по сети и узким местом с высокой вероятностью будет именно сеть. Кроме того, уменьшение надежности хранилища на хосте потенциально будет приводить к недоступности данных, тк в DRBD переключение со сломавшейся реплики на здоровую происходит не мгновенно.

## Рекомендации по использованию локальных дисков

DRBD использует сеть для репликации данных. При использовании NAS нагрузка на сеть будет расти кратно, так как узлы будут синхронизировать данные не только с NAS, но и между собой. Аналогично будет расти задержка на чтение или запись. NAS обычно предполагает использование RAID на своей стороны, что также увеличивает дополнительную нагрузку.
