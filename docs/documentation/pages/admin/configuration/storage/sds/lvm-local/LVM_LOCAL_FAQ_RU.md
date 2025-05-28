---
title: "Управление локальным хранилищем на основе LVM"
permalink: ru/admin/configuration/storage/sds/lvm-local-faq.html
lang: ru
---

## Выбор конкретных узлов для использования модуля

Чтобы ограничить использование модуля определёнными узлами кластера, необходимо задать метки в поле `nodeSelector` в настройках модуля.

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

Чтобы просмотреть текущие метки в поле `nodeSelector`, используйте следующую команду:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Пример вывода:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Модуль выбирает в качестве целевых только те узлы, у которых установлены все указанные в `nodeSelector` метки. Изменяя это поле, можно управлять списком узлов, на которых будет работать модуль.

{% alert level="warning" %}
В `nodeSelector` можно указать несколько меток. Однако для корректной работы модуля все эти метки должны присутствовать на каждом узле, на котором предполагается запуск `sds-local-volume-csi-node`.
{% endalert %}

После настройки меток, убедитесь, что на целевых узлах запущены поды `sds-local-volume-csi-node`. Проверить их наличие можно командой:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

## Проверка создания PVC на выбранном узле

Убедитесь, что на выбранном узле работает pod `sds-local-volume-csi-node`. Для этого выполните команду:

```shell
d8 k -n d8-sds-local-volume get po -owide
```

При отсутствии пода проверьте, что на узле установлены все метки, указанные в настройках модуля в поле `nodeSelector`. Подробнее об этом можно прочитать [здесь](#отсутствие-служебных-подов-на-нужном-узле).

## Вывод узла из-под управления модуля

Чтобы вывести узел из-под управления модуля, необходимо удалить метки, заданные в поле `nodeSelector` в настройках модуля `sds-local-volume`.

Для проверки текущих меток выполните команду:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Пример вывода:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Снимите указанные метки с узлов командой:

```shell
d8 k label node %node-name% %label-from-selector%-
```

{% alert level="warning" %}
После ключа метки необходимо сразу указать знак минуса для её удаления.
{% endalert %}

После этого под `sds-local-volume-csi-node` должен быть удален с узла. Проверьте состояние командой:

```shell
d8 k -n d8-sds-local-volume get po -owide
```

Если под после удаления метки остаётся, убедитесь, что метки из конфига `d8-sds-local-volume-controller-config` действительно удалены. Это можно проверить с помощью:

```shell
d8 k get node %node-name% --show-labels
```

Если метки отсутствуют, проверьте, что на узле не присутствуют [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) ресурсы, используемые [LocalStorageClass](../../../reference/cr/localstorageclass/) ресурсами. Подробнее об этой проверке можно прочитать [здесь](#проверка-зависимых-ресурсов-lvmvolumegroup-на-узле).

{% alert level="warning" %}
Обратите внимание, что на ресурсах [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) и [LocalStorageClass](../../../reference/cr/localstorageclass/), из-за которых не удается вывести узел из-под управления модуля, будет отображена метка `storage.deckhouse.io/sds-local-volume-candidate-for-eviction`.

На самом узле будет присутствовать метка `storage.deckhouse.io/sds-local-volume-need-manual-eviction`.
{% endalert %}

## Проверка зависимых ресурсов LVMVolumeGroup на узле

Для проверки зависимых ресурсов выполните следующие шаги:

1. Отобразить имеющиеся [LocalStorageClass](../../../reference/cr/localstorageclass/) ресурсы:

   ```shell
   d8 k get lsc
   ```

1. Проверить список используемых [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) ресурсов для конкретного [LocalStorageClass](../../../reference/cr/localstorageclass/):

   ```shell
   d8 k get lsc <LSC-NAME> -oyaml
   ```

   Пример вывода:

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

   > Обратите внимание на поле `spec.lvm.lvmVolumeGroups` — именно здесь указаны используемые ресурсы.

1. Отобразите список существующих [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) ресурсов:

   ```shell
   d8 k get lvg
   ```

   Пример вывода:

   ```console
   NAME              HEALTH        NODE            SIZE       ALLOCATED SIZE   VG        AGE
   lvg-on-worker-0   Operational   node-worker-0   40956Mi    0                test-vg   15d
   lvg-on-worker-1   Operational   node-worker-1   61436Mi    0                test-vg   15d
   lvg-on-worker-2   Operational   node-worker-2   122876Mi   0                test-vg   15d
   lvg-on-worker-3   Operational   node-worker-3   307196Mi   0                test-vg   15d
   lvg-on-worker-4   Operational   node-worker-4   307196Mi   0                test-vg   15d
   lvg-on-worker-5   Operational   node-worker-5   204796Mi   0                test-vg   15d
   ```

1. Убедитесь, что на узле, который планируется вывести из-под управления модуля, отсутствует любой [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) ресурс, используемый в [LocalStorageClass](../../../reference/cr/localstorageclass/)  ресурсах. Если такие ресурсы присутствуют, их необходимо удалить вручную, чтобы избежать потери контроля над томами.

## Оставшийся под sds-local-volume-csi-node после удаления меток

Если после удаления меток с узла pod `sds-local-volume-csi-node` продолжает работать, это, вероятнее всего, связано с наличием на узле [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) ресурсов, которые используются в одном из [LocalStorageClass](../../../reference/cr/localstorageclass/) ресурсов. Процесс проверки описан [выше](#проверка-зависимых-ресурсов-lvmvolumegroup-на-узле).

## Отсутствие служебных подов на нужном узле

Проблема может быть связана с некорректно установленными метками. Узлы, используемые модулем, определяются метками, заданными в поле `nodeSelector` в настройках модуля. Для просмотра текущих меток выполните:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Пример вывода:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Дополнительно можно проверить селекторы, используемые модулем, в конфигурации секрета `d8-sds-local-volume-controller-config` в пространстве имён `d8-sds-local-volume`:

```shell
d8 k -n d8-sds-local-volume get secret d8-sds-local-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
```

Пример вывода:

```yaml
nodeSelector:
  kubernetes.io/os: linux
  my-custom-label-key: my-custom-label-value
```

В выводе данной команды должны быть указаны все метки из настроек модуля `data.nodeSelector`, а также `kubernetes.io/os: linux`.

Проверьте метки на нужном узле:

```shell
d8 k get node %node-name% --show-labels
```

При необходимости добавьте недостающие метки на желаемый узел:

```shell
d8 k label node %node-name% my-custom-label-key=my-custom-label-value
```

Если метки присутствуют, проверьте наличие метки `storage.deckhouse.io/sds-local-volume-node=` на узле. Если метка отсутствует, убедитесь, что работает `sds-local-volume-controller`, и ознакомьтесь с его логами:

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

kubectl create -f migrate-job.yaml
kubectl -n $ns get jobs -o wide
kubectl_completed_check=0

echo "Waiting for data migration to be completed"
while [[ $kubectl_completed_check -eq 0 ]]; do
   kubectl -n $ns get pods | grep migrate-pv-$src
   sleep 5
   kubectl_completed_check=`kubectl -n $ns get pods | grep migrate-pv-$src | grep "Completed" | wc -l`
done
echo "Data migration completed"
```

Для запуска скрипта выполните команду:

```shell
migrate.sh NAMESPACE SOURCE_PVC_NAME DESTINATION_PVC_NAME
```
