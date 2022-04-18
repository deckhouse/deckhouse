---
title: "Модуль linstor: примеры конфигурации"
---

LINSTOR поддерживает несколько различных backend'ов для хранения данных, в основе которых лежат уже знакомые и проверенные временем технологии, такие как LVM и ZFS.  
Каждый том, созданный в LINSTOR, будет размещён на одном или нескольких узлах вашего кластера и реплицирован с помощью DRBD.

Мы стараемся делать Deckhouse максимально простым для использования и не хотим нагружать вас лишней информаций. Поэтому мы решили предоставить вам простой и знакомый интерфейс конфигурации LINSTOR.  
После включения модуля ваш кластер будет автоматически настроен. Останется только создать пулы хранения.

На данный момент мы поддерживаем два режима: **LVM** и **LVMThin**.

Каждый из них имеет свои достоинства и недостатки, подробнее о различиях читайте в [FAQ](faq.html).

Конфигурация LINSTOR в Deckhouse осуществляется посредством назначения специального тега `linstor-<имя_пула>` на LVM группу томов или тонкий пул.  
Теги должны быть уникальными в пределах одного узла. Поэтому каждый раз прежде чем назначить новый тег, убедитесь в отсутствии этого тега у других групп томов и тонких пулов выполнив следующие команды:
```shell
vgs -o name,tags
lvs -o name,vg_name,tags
```

* **LVM**

  Чтобы добавить LVM пул, создайте группу томов с тегом `linstor-<имя_пула>`, например, выполнив следующую команду:

  ```
  vgcreate linstor_data /dev/nvme0n1 /dev/nvme1n1 --add-tag linstor-data
  ```

  Вы также можете добавить существующую группу томов в LINSTOR, например, выполнив следующую команду:

  ```shell
  vgchange vg0 --add-tag linstor-data
  ```

* **LVMThin**

  Чтобы добавить LVMThin-пул, создайте тонкий пул LVM с тегом `linstor-<имя_пула>`, например, выполнив следующую команду:

  ```shell
  vgcreate linstor_data /dev/nvme0n1 /dev/nvme1n1
  lvcreate -L 1.8T -T linstor_data/thindata --add-tag linstor-thindata
  ```

  (обратите внимание: сама группа томов не обязана содержать какой-либо тег)

Используя приведённые выше команды создайте пулы хранения на всех узлах, где вы планируете хранить ваши данные.  
Используйте одинаковые имена пулов хранения на разных узлах, если хотите иметь общий StorageClass для всех них.

Когда все пулы хранения будут созданы, вы увидите три новых StorageClass'а:

```console
$ kubectl get storageclass
NAME                   PROVISIONER                  AGE
linstor-data-r1        linstor.csi.linbit.com       143s
linstor-data-r2        linstor.csi.linbit.com       142s
linstor-data-r3        linstor.csi.linbit.com       142s
```

Каждый из них можно использовать для создания томов с 1, 2 или 3 репликами в ваших пулах хранения.

При необходимости вы всегда можете обратиться к [расширенной конфигурации LINSTOR](advanced_usage.html), но мы крайне рекомендуем придерживаться этого упрощённого руководства.

## Data Locality

В случае гиперконвергентной инфраструктуры, вы можете захотеть чтобы ваши Pod'ы запускались на тех же узлах что и данные для них. Модуль **linstor** предоставляет кастомный kube-scheduler для таких задач.

Создайте Pod с указанием `schedulerName: linstor`, для того чтобы приоритизировать размещение Pod'а «поближе к данным» и получить максимальную производительность дисковой подсистемы.

Пример описания такого Pod'а:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: linstor
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
```

## Fencing

В случае если ваше приложение не умеет работать в режиме высокой доступности, вы можете добавить специальную аннотацию, которая позволит **linstor** автоматически удалять Pod вашего приложения с "проблемного" узла. Это позволит Kubernetes безопасно перезапустить ваше приложение на новом узле.

Пример описания StatefulSet с такой аннотацией:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-stateful-app
spec:
  serviceName: my-stateful-app
  selector:
    matchLabels:
      app.kubernetes.io/name: my-stateful-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-stateful-app
        linstor.csi.linbit.com/on-storage-lost: remove
    ...
```
