---
title: "Управление локальным хранилищем на основе LVM"
permalink: ru/admin/configuration/storage/sds/lvm-local-faq.html
lang: ru
---

## Выбор конкретных узлов для использования модуля

Чтобы ограничить использование модуля определёнными узлами кластера, необходимо задать лейблы в [поле `nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) в настройках модуля.

Для отображения и редактирования настроек модуля выполните команду:

```shell
d8 k edit mc sds-local-volume
```

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-local-volume
spec:
  enabled: true
  settings:
    dataNodes:
      nodeSelector:
        my-custom-label-key: my-custom-label-value
status:
  message: ""
  version: "1"
```

Чтобы просмотреть текущие лейблы в поле `nodeSelector`, используйте следующую команду:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Пример вывода:

```console
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Модуль выбирает в качестве целевых только те узлы, у которых установлены все указанные в `nodeSelector` лейблы. Изменяя это поле, можно управлять списком узлов, на которых будет работать модуль.

{% alert level="warning" %}
В `nodeSelector` можно указать несколько лейблов. Однако для корректной работы модуля все эти лейблы должны присутствовать на каждом узле, на котором предполагается запуск `sds-local-volume-csi-node`.
{% endalert %}

После настройки лейблов убедитесь, что на целевых узлах запущены поды `sds-local-volume-csi-node`. Проверить их наличие можно командой:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

## Проверка создания PVC на выбранном узле

Убедитесь, что на выбранном узле работает pod `sds-local-volume-csi-node`. Для этого выполните команду:

```shell
d8 k -n d8-sds-local-volume get po -owide
```

При отсутствии пода проверьте, что на узле установлены все лейблы, указанные в настройках модуля в поле `nodeSelector`. Подробнее о способах решения проблемы с отсутствием подов на нужном узле можно прочитать [в этом разделе](#отсутствие-служебных-подов-на-нужном-узле).

## Вывод узла из-под управления модуля

Чтобы вывести узел из-под управления модуля, необходимо удалить лейблы, заданные в [поле `nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) в настройках модуля `sds-local-volume`.

Для проверки текущих лейблов выполните команду:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Пример вывода:

```console
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Снимите указанные лейблы с узлов командой:

```shell
d8 k label node %node-name% %label-from-selector%-
```

{% alert level="warning" %}
После ключа лейбла необходимо указать знак минуса для её удаления.
{% endalert %}

После этого под `sds-local-volume-csi-node` должен быть удален с узла. Проверьте его состояние командой:

```shell
d8 k -n d8-sds-local-volume get po -owide
```

Если под остаётся после удаления метки, убедитесь, что метки из конфигурации `d8-sds-local-volume-controller-config` действительно удалены. Это можно проверить с помощью следующей команды:

```shell
d8 k get node %node-name% --show-labels
```

Если лейблы отсутствуют, проверьте, что на узле нет ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), используемых ресурсами [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass). Подробнее об этой проверке можно прочитать [в этом разделе](#проверка-зависимых-ресурсов-lvmvolumegroup-на-узле).

{% alert level="warning" %}
Обратите внимание, что для ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) и [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass), из-за которых не удается вывести узел из-под управления модуля, будет назначен лейбл `storage.deckhouse.io/sds-local-volume-candidate-for-eviction`.

На самом узле будет присутствовать лейбл `storage.deckhouse.io/sds-local-volume-need-manual-eviction`.
{% endalert %}

## Проверка зависимых ресурсов LVMVolumeGroup на узле

Для проверки зависимых ресурсов выполните следующие шаги:

1. Отобразите имеющиеся ресурсы [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass):

   ```shell
   d8 k get lsc
   ```

1. Проверьте у каждого из них список используемых ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup).

   Вы можете сразу отобразить содержимое всех [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) ресурсов, выполнив команду:

   ```shell
   d8 k get lsc -oyaml
   ```

   Примерный вид [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass):

   ```yaml
   apiVersion: v1
   items:
   - apiVersion: storage.deckhouse.io/v1alpha1
     kind: LocalStorageClass
     metadata:
       finalizers:
       - localstorageclass.storage.deckhouse.io
       name: test-sc
     spec:
       lvm:
         lvmVolumeGroups:
         - name: test-vg
         type: Thick
       reclaimPolicy: Delete
       volumeBindingMode: WaitForFirstConsumer
     status:
       phase: Created
   kind: List
   ```

   Обратите внимание на поле `spec.lvm.lvmVolumeGroups` — именно в нем указаны используемые ресурсы [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup).

1. Отобразите список существующих ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup):

   ```shell
   d8 k get lvg
   ```

   Примерный вывод [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup):

   ```text
   NAME              HEALTH        NODE            SIZE       ALLOCATED SIZE   VG        AGE
   lvg-on-worker-0   Operational   node-worker-0   40956Mi    0                test-vg   15d
   lvg-on-worker-1   Operational   node-worker-1   61436Mi    0                test-vg   15d
   lvg-on-worker-2   Operational   node-worker-2   122876Mi   0                test-vg   15d
   lvg-on-worker-3   Operational   node-worker-3   307196Mi   0                test-vg   15d
   lvg-on-worker-4   Operational   node-worker-4   307196Mi   0                test-vg   15d
   lvg-on-worker-5   Operational   node-worker-5   204796Mi   0                test-vg   15d
   ```

1. Проверьте, что на узле, который вы собираетесь вывести из-под управления модуля, не присутствует какой-либо ресурс [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), используемый в ресурсах [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass). Во избежание непредвиденной потери контроля за уже созданными с помощью модуля томами вручную удалите зависимые ресурсы, совершив необходимые операции над томом.

## Оставшийся под sds-local-volume-csi-node после удаления лейблов

Если после удаления лейблов с узла под `sds-local-volume-csi-node` продолжает работать, это, вероятнее всего, связано с наличием на узле ресурсов [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup), которые используются в одном из ресурсов [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass). Процесс проверки описан [выше](#проверка-зависимых-ресурсов-lvmvolumegroup-на-узле).

## Отсутствие служебных подов на нужном узле

Проблема может быть связана с некорректно установленными лейблами. Узлы, используемые модулем, определяются лейблами, заданными в [поле `nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) в настройках модуля. Для просмотра текущих лейблов выполните:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Пример вывода:

```console
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Дополнительно можно проверить селекторы, используемые модулем, в конфигурации секрета `d8-sds-local-volume-controller-config` в пространстве имён `d8-sds-local-volume`:

```shell
d8 k -n d8-sds-local-volume get secret d8-sds-local-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
```

Пример вывода:

```console
nodeSelector:
  kubernetes.io/os: linux
  my-custom-label-key: my-custom-label-value
```

В выводе данной команды должны быть указаны все лейблы из настроек модуля `data.nodeSelector`, а также `kubernetes.io/os: linux`.

Проверьте лейблы на нужном узле:

```shell
d8 k get node %node-name% --show-labels
```

При необходимости добавьте недостающие лейблы на желаемый узел:

```shell
d8 k label node %node-name% my-custom-label-key=my-custom-label-value
```

Если метки присутствуют, проверьте наличие лейбла `storage.deckhouse.io/sds-local-volume-node=` на узле. Если лейбл отсутствует, убедитесь, что работает `sds-local-volume-controller`, и ознакомьтесь с его логами:

```shell
d8 k -n d8-sds-local-volume get po -l app=sds-local-volume-controller
d8 k -n d8-sds-local-volume logs -l app=sds-local-volume-controller
```

## Перемещение данных между PVC

Скопируйте следующий скрипт в файл `migrate.sh` на любом master-узле:

```shell
#!/bin/bash

ns=$1
src=$2
dst=$3

if [[ -z $3 ]]; then
  echo "You must give as args: namespace source_pvc_name destination_pvc_name"
  exit 1
fi

echo "Creating job yaml"
cat > migrate-job.yaml << EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate-pv-$src
  namespace: $ns
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: debian
        command: [ "/bin/bash", "-c" ]
        args:
          -
            apt-get update && apt-get install -y rsync &&
            ls -lah /src_vol /dst_vol &&
            df -h &&
            rsync -avPS --delete /src_vol/ /dst_vol/ &&
            ls -lah /dst_vol/ &&
            du -shxc /src_vol/ /dst_vol/
        volumeMounts:
        - mountPath: /src_vol
          name: src
          readOnly: true
        - mountPath: /dst_vol
          name: dst
      restartPolicy: Never
      volumes:
      - name: src
        persistentVolumeClaim:
          claimName: $src
      - name: dst
        persistentVolumeClaim:
          claimName: $dst
  backoffLimit: 1
EOF

d8 k create -f migrate-job.yaml
d8 k -n $ns get jobs -o wide
kubectl_completed_check=0

echo "Waiting for data migration to be completed"
while [[ $kubectl_completed_check -eq 0 ]]; do
   d8 k -n $ns get pods | grep migrate-pv-$src
   sleep 5
   kubectl_completed_check=`d8 k -n $ns get pods | grep migrate-pv-$src | grep "Completed" | wc -l`
done
echo "Data migration completed"
```

Для запуска скрипта выполните команду:

```shell
migrate.sh NAMESPACE SOURCE_PVC_NAME DESTINATION_PVC_NAME
```

## Создание снимков томов

Подробную информацию о снимках и используемых ресурсах можно найти [в документации Kubernetes](https://kubernetes.io/docs/concepts/storage/volume-snapshots/).

1. Включите [модуль `snapshot-controller`](/modules/snapshot-controller/):

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: snapshot-controller
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Теперь вы можете создавать снимки томов. Для этого выполните следующую команду с необходимыми параметрами:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: my-snapshot
     namespace: <name of the namespace where the PVC is located>
   spec:
     volumeSnapshotClassName: sds-local-volume-snapshot-class
     source:
       persistentVolumeClaimName: <name of the PVC to snapshot>
   EOF
   ```

   Обратите внимание, что `sds-local-volume-snapshot-class` создается автоматически, и его `deletionPolicy` установлена в `Delete`, что означает, что ресурс VolumeSnapshotContent будет удален при удалении связанного ресурса VolumeSnapshot.

1. Чтобы проверить статус созданного снимка, выполните команду:

   ```shell
   d8 k get volumesnapshot
   ```

   Данная команда выведет список всех снимков и их текущий статус.
