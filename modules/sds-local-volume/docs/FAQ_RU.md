---
title: "Модуль sds-local-volume: FAQ"
description: "Модуль sds-local-volume: FAQ"
---

## Когда следует использовать LVM, а когда LVMThin?

- LVM проще и обладает высокой производительностью, сравнимой с производительностью накопителя;
- LVMThin позволяет использовать overprovisioning, но производительность ниже, чем у LVM.

{{< alert level="warning" >}}
Overprovisioning в LVMThin нужно использовать с осторожностью, контроллируя наличие свободного места в пуле (В системе мониторинга кластера есть отдельные события при достижении 20%, 10%, 5% и 1% свободного места в пуле)

При отсутствии свободного места в пуле будет наблюдатся деградация в работе модуля в целом, а также существует реальная вероятность потери данных!
{{< /alert >}}

## Как назначить StorageClass по умолчанию?

Добавьте аннотацию `storageclass.kubernetes.io/is-default-class: "true"` в соответствующий ресурс StorageClass:

```shell
kubectl annotate storageclasses.storage.k8s.io <storageClassName> storageclass.kubernetes.io/is-default-class=true
```

## Я не хочу, чтобы модуль использовался на всех узлах кластера. Как мне выбрать желаемые узлы?

Узлы, которые будут задействованы модулем, определяются специальными метками, указанными в поле `nodeSelector` в настройках модуля.

Для отображения и редактирования настроек модуля, можно выполнить команду:

```shell
kubectl edit mc sds-local-volume
```

Примерный вывод команды:

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

Для отображения существующих меток, указанных в поле `nodeSelector`, можно выполнить команду:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Примерный вывод команды:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Узлы, метки которых включают в себя набор, указанный в настройках, выбираются модулем как целевые для использования. Соответственно, изменяя поле `nodeSelector` Вы можете влиять на список узлов, которые будут использованы модулем.

{{< alert level="warning" >}}
В поле `nodeSelector` может быть указано любое количество меток, но важно, чтобы каждая из указанных меток присутствовала на узле, который Вы собираетесь использовать для работы с модулем. Именно при наличии всех указанных меток на выбранном узле, произойдет запуск pod-а `sds-local-volume-csi-node`.
{{< /alert >}}

После добавление меток на узлах должны быть запущены pod-ы `sds-local-volume-csi-node`. Проверить их наличие можно командой:

```shell
 kubectl -n d8-sds-local-volume get pod -owide
 ```

## Почему не удается создать PVC на выбранном узле с помощью модуля?

Пожалуйста, проверьте, что на выбранном узле работает pod `sds-local-volume-csi-node`.

```shell
kubectl -n d8-sds-local-volume get po -owide
```

Если pod отсутствует, пожалуйста, убедитесь, что на выбранном узле присутствуют все метки, указанные в настройках модуля в поле `nodeSelector`. Подробнее об этом [здесь](#служебные-pod-ы-компонентов-sds-local-volume-не-создаются-на-нужном-мне-узле-почему).

## Я хочу вывести узел из-под управления модуля, что делать?

Для вывода узла из-под управления модуля необходимо убрать метки, указанные в поле `nodeSelector` в настройках модуля `sds-local-volume`.

Проверить наличие существующих меток в `nodeSelector` можно командой:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Примерный вывод команды:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Снимите указанные в `nodeSelector` метки с желаемых узлов.

```shell
kubectl label node %node-name% %label-from-selector%-
```

{{< alert level="warning" >}}
Для снятия метки необходимо после его ключа вместо значения сразу же поставить знак минуса.
{{< /alert >}}

В результате pod `sds-local-volume-csi-node` должен быть удален с желаемого узла. Для проверки состояния можно выполнить команду:

```shell
kubectl -n d8-sds-local-volume get po -owide
```

Если pod `sds-local-volume-csi-node` после удаления метки `nodeSelector` все же остался на узле, пожалуйста, убедитесь, что указанные в конфиге `d8-sds-local-volume-controller-config` в `nodeSelector` метки действительно успешно снялись с выбранного узла.
Проверить это можно командой:

```shell
kubectl get node %node-name% --show-labels
```

Если метки из `nodeSelector` не присутствуют на узле, то убедитесь, что данному узлу не принадлежат `LVMVolumeGroup` ресурсы, использующиеся `LocalStorageClass` ресурсами. Подробнее об этой проверке можно прочитать [здесь](#как-проверить-имеются-ли-зависимые-ресурсы-lvmvolumegroup-на-узле).

{{< alert level="warning" >}}
Обратите внимание, что на ресурсах `LVMVolumeGroup` и `LocalStorageClass`, из-за которых не удается вывести узел из-под управления модуля будет отображена метка `storage.deckhouse.io/sds-local-volume-candidate-for-eviction`.

На самом узле будет присутствовать метка `storage.deckhouse.io/sds-local-volume-need-manual-eviction`.
{{< /alert >}}

## Как проверить, имеются ли зависимые ресурсы `LVMVolumeGroup` на узле?

Для проверки таковых ресурсов необходимо выполнить следующие шаги:
1. Отобразить имеющиеся `LocalStorageClass` ресурсы

   ```shell
   kubectl get lsc
   ```

2. Проверить у каждого из них список используемых `LVMVolumeGroup` ресурсов

   > Вы можете сразу отобразить содержимое всех `LocalStorageClass` ресурсов, выполнив команду:
   >
   > ```shell
   > kubectl get lsc -oyaml
   > ```

   ```shell
   kubectl get lsc %lsc-name% -oyaml
   ```

   Примерный вид `LocalStorageClass`

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

   > Обратите внимание на поле spec.lvm.lvmVolumeGroups - именно в нем указаны используемые `LVMVolumeGroup` ресурсы.

3. Отобразите список существующих `LVMVolumeGroup` ресурсов

   ```shell
   kubectl get lvg
   ```

   Примерный вывод `LVMVolumeGroup` ресурсов:

   ```text
   NAME              HEALTH        NODE                         SIZE       ALLOCATED SIZE   VG        AGE
   lvg-on-worker-0   Operational   node-worker-0   40956Mi    0                test-vg   15d
   lvg-on-worker-1   Operational   node-worker-1   61436Mi    0                test-vg   15d
   lvg-on-worker-2   Operational   node-worker-2   122876Mi   0                test-vg   15d
   lvg-on-worker-3   Operational   node-worker-3   307196Mi   0                test-vg   15d
   lvg-on-worker-4   Operational   node-worker-4   307196Mi   0                test-vg   15d
   lvg-on-worker-5   Operational   node-worker-5   204796Mi   0                test-vg   15d
   ```

4. Проверьте, что на узле, который вы собираетесь вывести из-под управления модуля, не присутствует какой-либо `LVMVolumeGroup` ресурс, используемый в `LocalStorageClass` ресурсах.

   Во избежание непредвиденной потери контроля за уже созданными с помощью модуля томами пользователю необходимо вручную удалить зависимые ресурсы, совершив необходимые операции над томом.

## Я убрал метки с узла, но pod `sds-local-volume-csi-node` остался. Почему так произошло?

Вероятнее всего, на узле присутствуют `LVMVolumeGroup` ресурсы, которые используются в одном из `LocalStorageClass` ресурсов.

Во избежание непредвиденной потери контроля за уже созданными с помощью модуля томами пользователю необходимо вручную удалить зависимые ресурсы, совершив необходимые операции над томом.

Процесс проверки на наличие вышеуказанных ресурсов описан [здесь](#как-проверить-имеются-ли-зависимые-ресурсы-lvmvolumegroup-на-узле).

## Служебные pod-ы компонентов `sds-local-volume` не создаются на нужном мне узле. Почему?

С высокой вероятностью проблемы связаны с метками на узле.

Узлы, которые будут задействованы модулем, определяются специальными метками, указанными в поле `nodeSelector` в настройках модуля.

Для отображения существующих меток, указанных в поле `nodeSelector`, можно выполнить команду:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Примерный вывод команды:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Узлы, метки которых включают в себя набор, указанный в настройках, выбираются модулем как целевые для использования.

Также Вы можете дополнительно проверить селекторы, которые используются модулем в конфиге секрета `d8-sds-local-volume-controller-config` в пространстве имен `d8-sds-local-volume`.

```shell
kubectl -n d8-sds-local-volume get secret d8-sds-local-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
```

Примерный вывод команды:

```yaml
nodeSelector:
  kubernetes.io/os: linux
  my-custom-label-key: my-custom-label-value
```

В выводе данной команды должны быть указаны все метки из настроек модуля `data.nodeSelector`, а также `kubernetes.io/os: linux`.

Проверьте метки на нужном вам узле:

```shell
kubectl get node %node-name% --show-labels
```

При необходимости добавьте недостающие метки на желаемый узел:

```shell
kubectl label node %node-name% my-custom-label-key=my-custom-label-value
```

Если метки присутствуют, необходимо проверить наличие метки `storage.deckhouse.io/sds-local-volume-node=` на узле. Если метка отсутствует, следует проверить работает ли `sds-local-volume-controller`, и в случае его работоспособности, проверить логи:

```shell
kubectl -n d8-sds-local-volume get po -l app=sds-local-volume-controller
kubectl -n d8-sds-local-volume logs -l app=sds-local-volume-controller
```

## Как переместить данные между PVC?

Скопируйте следующий скрипт в файл migrate.sh на любом master узле.
Использование: migrate.sh NAMESPACE SOURCE_PVC_NAME DESTINATION_PVC_NAME

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
