## Миграция с in-tree RBD драйвера на CSI (Ceph CSI)

Для упрощения процесса миграции был написан скрипт [rbd-in-tree-to-ceph-csi-migration-helper.sh](../../tools/rbd-in-tree-to-ceph-csi-migration-helper.sh).
Перед запуском необходимо удалить Pod использующий PVC. В процессе миграции будет необходимо вручную выполнить команду в ceph-кластере для переименования rbd-образа (Ceph CSI использует другой формат имени).

Во время работы скрипта будет сделан бэкап манифестов мигрируемых PVC и PV, далее они будут удалены и в итоге будут созданы новые PVC и PV. Удаление PV не приведет к удалению rbd-образа в ceph-кластере, т.к. перед удалением PV он будет переименован.

Для работы скрипта небходимы PVC и PV, из манифестов которых будут позаимствованы параметры характерные для Ceph CSI. Можно создать их используя следующий манифест:
```yaml
kubectl create -f - <<"END"
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: sample
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: <ceph-csi-storage-class-name-change-me>
END
```

Пример работы скрипта:
```bash
root@kube-master-0:~# ./rbd-migrator.sh default/sample default/data-test-0
Rename the rbd image in your ceph cluster using the following command:
>rbd mv kube/kubernetes-dynamic-pvc-162a2c43-568e-40ab-aedb-a4632a613ecd kube/csi-vol-162a2c43-568e-40ab-aedb-a4632a613ecd
After renaming, enter yes to confirm: yes
PersistentVolumeClaim data-test-0 and PersistentVolume pvc-4a77a995-ce1e-463c-9726-d05966d3c5ef will be removed (Type yes to confirm): yes
>kubectl -n default delete pvc data-test-0
persistentvolumeclaim "data-test-0" deleted
>kubectl delete pv pvc-4a77a995-ce1e-463c-9726-d05966d3c5ef
persistentvolume "pvc-4a77a995-ce1e-463c-9726-d05966d3c5ef" deleted
>kubectl create -f - <<"END"
{
  "apiVersion": "v1",
  "kind": "PersistentVolumeClaim",
  "metadata": {
    "annotations": {
      "pv.kubernetes.io/bind-completed": "yes",
      "pv.kubernetes.io/bound-by-controller": "yes",
      "volume.beta.kubernetes.io/storage-provisioner": "rbd.csi.ceph.com"
    },
    "finalizers": [
      "kubernetes.io/pvc-protection"
    ],
    "labels": {
      "app": "test"
    },
    "name": "data-test-0",
    "namespace": "default"
  },
  "spec": {
    "accessModes": [
      "ReadWriteOnce"
    ],
    "resources": {
      "requests": {
        "storage": "1Gi"
      }
    },
    "storageClassName": "ceph-csi-rbd",
    "volumeMode": "Filesystem",
    "volumeName": "pvc-4a77a995-ce1e-463c-9726-d05966d3c5ef"
  }
}
END
Apply this manifest in the cluster? (Type yes to confirm): yes
persistentvolumeclaim/data-test-0 created
>kubectl create -f - <<"END"
{
  "apiVersion": "v1",
  "kind": "PersistentVolume",
  "metadata": {
    "annotations": {
      "pv.kubernetes.io/provisioned-by": "rbd.csi.ceph.com",
      "volume.kubernetes.io/provisioner-deletion-secret-name": "csi-new",
      "volume.kubernetes.io/provisioner-deletion-secret-namespace": "d8-ceph-csi"
    },
    "finalizers": [
      "kubernetes.io/pv-protection"
    ],
    "name": "pvc-4a77a995-ce1e-463c-9726-d05966d3c5ef"
  },
  "spec": {
    "accessModes": [
      "ReadWriteOnce"
    ],
    "capacity": {
      "storage": "1Gi"
    },
    "claimRef": {
      "apiVersion": "v1",
      "kind": "PersistentVolumeClaim",
      "name": "data-test-0",
      "namespace": "default",
      "resourceVersion": "14908531",
      "uid": "0ac58d43-75f9-4481-96fd-dcf8ca60ad85"
    },
    "mountOptions": [
      "discard"
    ],
    "persistentVolumeReclaimPolicy": "Retain",
    "storageClassName": "ceph-csi-rbd",
    "volumeMode": "Filesystem",
    "csi": {
      "controllerExpandSecretRef": {
        "name": "csi-new",
        "namespace": "d8-ceph-csi"
      },
      "driver": "rbd.csi.ceph.com",
      "fsType": "ext4",
      "nodeStageSecretRef": {
        "name": "csi-new",
        "namespace": "d8-ceph-csi"
      },
      "volumeAttributes": {
        "clusterID": "60f356ee-7c2d-4556-81be-c24b34a30b2a",
        "imageFeatures": "layering",
        "imageName": "csi-vol-162a2c43-568e-40ab-aedb-a4632a613ecd",
        "journalPool": "kube",
        "pool": "kube",
        "storage.kubernetes.io/csiProvisionerIdentity": "1666697721019-8081-rbd.csi.ceph.com"
      },
      "volumeHandle": "0001-0024-60f356ee-7c2d-4556-81be-c24b34a30b2a-0000000000000005-162a2c43-568e-40ab-aedb-a4632a613ecd"
    }
  }
}
END
Apply this manifest in the cluster? (Type yes to confirm): yes
persistentvolume/pvc-4a77a995-ce1e-463c-9726-d05966d3c5ef created
```


## Описание процесса миграции

### Оглавление:

1. [Манифесты мигрируемых PVC и PV используемые для демонстрации процесса.](#%D0%BC%D0%B0%D0%BD%D0%B8%D1%84%D0%B5%D1%81%D1%82%D1%8B-%D0%BC%D0%B8%D0%B3%D1%80%D0%B8%D1%80%D1%83%D0%B5%D0%BC%D1%8B%D1%85-pvc-%D0%B8-pv)
2. [Манифесты PVC и PV из которых будут заимствованы параметры характерные для Ceph CSI.](#%D0%BC%D0%B0%D0%BD%D0%B8%D1%84%D0%B5%D1%81%D1%82%D1%8B-pvc-%D0%B8-pv-%D0%B8%D0%B7-%D0%BA%D0%BE%D1%82%D0%BE%D1%80%D1%8B%D1%85-%D0%B1%D1%83%D0%B4%D1%83%D1%82-%D0%B7%D0%B0%D0%B8%D0%BC%D1%81%D1%82%D0%B2%D0%BE%D0%B2%D0%B0%D0%BD%D1%8B-%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B-%D1%85%D0%B0%D1%80%D0%B0%D0%BA%D1%82%D0%B5%D1%80%D0%BD%D1%8B%D0%B5-%D0%B4%D0%BB%D1%8F-ceph-csi)
3. [Переименование RBD-образа в ceph-кластере.](#%D0%BF%D0%B5%D1%80%D0%B5%D0%B8%D0%BC%D0%B5%D0%BD%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D0%B5-rbd-%D0%BE%D0%B1%D1%80%D0%B0%D0%B7%D0%B0-%D0%B2-ceph-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B5)
4. [Удаление PVC и PV из кластера.](#%D1%83%D0%B4%D0%B0%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-pvc-%D0%B8-pv-%D0%B8%D0%B7-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B0)
5. [Генерация нового манифеста PVC и создание объекта в кластере.](#%D0%B3%D0%B5%D0%BD%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D1%8F-%D0%BD%D0%BE%D0%B2%D0%BE%D0%B3%D0%BE-%D0%BC%D0%B0%D0%BD%D0%B8%D1%84%D0%B5%D1%81%D1%82%D0%B0-pvc-%D0%B8-%D1%81%D0%BE%D0%B7%D0%B4%D0%B0%D0%BD%D0%B8%D0%B5-%D0%BE%D0%B1%D1%8A%D0%B5%D0%BA%D1%82%D0%B0-%D0%B2-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B5)
6. [Генерация нового манифеста PV и создание объекта в кластере.](#%D0%B3%D0%B5%D0%BD%D0%B5%D1%80%D0%B0%D1%86%D0%B8%D1%8F-%D0%BD%D0%BE%D0%B2%D0%BE%D0%B3%D0%BE-%D0%BC%D0%B0%D0%BD%D0%B8%D1%84%D0%B5%D1%81%D1%82%D0%B0-pv-%D0%B8-%D1%81%D0%BE%D0%B7%D0%B4%D0%B0%D0%BD%D0%B8%D0%B5-%D0%BE%D0%B1%D1%8A%D0%B5%D0%BA%D1%82%D0%B0-%D0%B2-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B5)


### Манифесты мигрируемых PVC и PV:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: kubernetes.io/rbd
  creationTimestamp: "2022-11-03T13:15:43Z"
  finalizers:
  - kubernetes.io/pvc-protection
  labels:
    app: test
  name: data-test-0
  namespace: default
  resourceVersion: "8956688"
  uid: cd6f7b26-d768-4cab-88a4-baca5b242cc5
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: rbd
  volumeMode: Filesystem
  volumeName: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
status:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  phase: Bound
```
```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    kubernetes.io/createdby: rbd-dynamic-provisioner
    pv.kubernetes.io/bound-by-controller: "yes"
    pv.kubernetes.io/provisioned-by: kubernetes.io/rbd
  creationTimestamp: "2022-11-03T13:15:49Z"
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
  resourceVersion: "8956671"
  uid: 4ab7fcf4-e8db-426e-a7aa-f5380ef857c7
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: data-test-0
    namespace: default
    resourceVersion: "8956643"
    uid: cd6f7b26-d768-4cab-88a4-baca5b242cc5
  mountOptions:
  - discard
  persistentVolumeReclaimPolicy: Delete
  rbd:
    image: kubernetes-dynamic-pvc-f32fea79-d658-4ab1-967a-fb6e8f930dec
    keyring: /etc/ceph/keyring
    monitors:
    - 192.168.4.215:6789
    pool: kube
    secretRef:
      name: ceph-secret
    user: kube
  storageClassName: rbd
  volumeMode: Filesystem
status:
  phase: Bound
```

### Манифесты PVC и PV из которых будут заимствованы параметры характерные для Ceph CSI.

Используется StorageClass соданный модулем ceph-csi.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: rbd.csi.ceph.com
  creationTimestamp: "2022-11-03T12:46:20Z"
  finalizers:
  - kubernetes.io/pvc-protection
  name: sample
  namespace: default
  resourceVersion: "8950577"
  uid: abdbb7ea-5da6-47f3-8b76-b968a93b7bc1
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: new-rbd
  volumeMode: Filesystem
  volumeName: pvc-abdbb7ea-5da6-47f3-8b76-b968a93b7bc1
status:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  phase: Bound
```

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    pv.kubernetes.io/provisioned-by: rbd.csi.ceph.com
    volume.kubernetes.io/provisioner-deletion-secret-name: csi-new
    volume.kubernetes.io/provisioner-deletion-secret-namespace: d8-ceph-csi
  creationTimestamp: "2022-11-03T12:46:27Z"
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-abdbb7ea-5da6-47f3-8b76-b968a93b7bc1
  resourceVersion: "8950562"
  uid: 6200ce15-b6f2-45af-94d0-828913e850d0
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: sample
    namespace: default
    resourceVersion: "8950550"
    uid: abdbb7ea-5da6-47f3-8b76-b968a93b7bc1
  csi:
    controllerExpandSecretRef:
      name: csi-new
      namespace: d8-ceph-csi
    driver: rbd.csi.ceph.com
    fsType: ext4
    nodeStageSecretRef:
      name: csi-new
      namespace: d8-ceph-csi
    volumeAttributes:
      clusterID: 60f356ee-7c2d-4556-81be-c24b34a30b2a
      imageFeatures: layering
      imageName: csi-vol-880ec27e-5b75-11ed-a252-fa163ee74632
      journalPool: kube
      pool: kube
      storage.kubernetes.io/csiProvisionerIdentity: 1666697721019-8081-rbd.csi.ceph.com
    volumeHandle: 0001-0024-60f356ee-7c2d-4556-81be-c24b34a30b2a-0000000000000005-880ec27e-5b75-11ed-a252-fa163ee74632
  mountOptions:
  - discard
  persistentVolumeReclaimPolicy: Delete
  storageClassName: new-rbd
  volumeMode: Filesystem
status:
  phase: Bound
```

### Переименование RBD-образа в ceph-кластере

Необходимо, поскольку Ceph CSI драйвер использует другой формат имени rbd-образа.
```shell
rbd mv kube/kubernetes-dynamic-pvc-<rbd-image-uid> kube/csi-vol-<rbd-image-uid>
```
* `kube` - имя пула в ceph-кластере;
* `kubernetes-dynamic-pvc-<uid>` - формат имени rbd-образа используемый in-tree драйвером;
* `csi-vol-<uid>` - формат имени rbd-образа используемый Ceph CSI.


### Удаление PVC и PV из кластера

```bash
kubectl -n default delete pvc data-test-0
kubectl delete pv pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
```
Т.к. на предыдущем шаге мы переименовали rbd-образ в ceph-кластере, то удаление PersistentVolume не повлечет удаление образа.

### Генерация нового манифеста PVC и создание объекта в кластере

Исходный манифест с комментариями:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: kubernetes.io/rbd # заменим аннотацию на аналогичную из PVC sample
  creationTimestamp: "2022-11-03T13:15:43Z"  # удалим
  finalizers:
  - kubernetes.io/pvc-protection
  labels:
    app: test
  name: data-test-0
  namespace: default
  resourceVersion: "8956688"  # удалим
  uid: cd6f7b26-d768-4cab-88a4-baca5b242cc5 # удалим
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: rbd
  volumeMode: Filesystem
  volumeName: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
status: # удалим
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  phase: Bound
```

В результате получится:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: rbd.csi.ceph.com
  finalizers:
  - kubernetes.io/pvc-protection
  labels:
    app: test
  name: data-test-0
  namespace: default
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: ceph-csi-rbd
  volumeMode: Filesystem
  volumeName: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
```

Создадим объект в кластере используя этот манифест.

### Генерация нового манифеста PV и создание объекта в кластере.

Исходный манифест с комментариями:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    kubernetes.io/createdby: rbd-dynamic-provisioner
    pv.kubernetes.io/bound-by-controller: "yes"
    pv.kubernetes.io/provisioned-by: kubernetes.io/rbd
  creationTimestamp: "2022-11-03T13:15:49Z" # удалим
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
  resourceVersion: "8956671" # удалим
  uid: 4ab7fcf4-e8db-426e-a7aa-f5380ef857c7 # удалим
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: data-test-0
    namespace: default
    resourceVersion: "8956643" # заменим на новый из созданного PVC на предыдущем шаге
    uid: cd6f7b26-d768-4cab-88a4-baca5b242cc5 # заменим на новый из созданного PVC на предыдущем шаге
  mountOptions:
  - discard
  persistentVolumeReclaimPolicy: Delete
  rbd: # удалим
    image: kubernetes-dynamic-pvc-f32fea79-d658-4ab1-967a-fb6e8f930dec
    keyring: /etc/ceph/keyring
    monitors:
    - 192.168.4.215:6789
    pool: kube
    secretRef:
      name: ceph-secret
    user: kube
  storageClassName: rbd # заменим на ceph-csi-rbd
  volumeMode: Filesystem
  # добавим секцию csi
status: # удалим
  phase: Bound
```

Образец `spec.csi` берём из созданного ранее PV:

```yaml
  csi:
    controllerExpandSecretRef:
      name: csi-new
      namespace: d8-ceph-csi
    driver: rbd.csi.ceph.com
    fsType: ext4
    nodeStageSecretRef:
      name: csi-new
      namespace: d8-ceph-csi
    volumeAttributes:
      clusterID: 60f356ee-7c2d-4556-81be-c24b34a30b2a
      imageFeatures: layering
      imageName: csi-vol-880ec27e-5b75-11ed-a252-fa163ee74632 # заменим uid
      journalPool: kube
      pool: kube
      storage.kubernetes.io/csiProvisionerIdentity: 1666697721019-8081-rbd.csi.ceph.com
    volumeHandle: 0001-0024-60f356ee-7c2d-4556-81be-c24b34a30b2a-0000000000000005-880ec27e-5b75-11ed-a252-fa163ee74632 # заменим uid
```

В полях `imageName` и `volumeHandle` заменим uid rbd-образа.

Для наглядности ниже uid выделен тегами `<uid>here<uid>`:

```yaml
imageName: csi-vol-<uid>880ec27e-5b75-11ed-a252-fa163ee74632<uid>
volumeHandle: 0001-0024-60f356ee-7c2d-4556-81be-c24b34a30b2a-0000000000000005-<uid>880ec27e-5b75-11ed-a252-fa163ee74632<uid>
```

В итоге получится манифест:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    pv.kubernetes.io/provisioned-by: rbd.csi.ceph.com
    volume.kubernetes.io/provisioner-deletion-secret-name: csi-new
    volume.kubernetes.io/provisioner-deletion-secret-namespace: d8-ceph-csi
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
spec:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 1Gi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: data-test-0
    namespace: default
    resourceVersion: "8956721"
    uid: cd6f7b26-d768-4cab-88a4-baca5b242cc7
  mountOptions:
  - discard
  persistentVolumeReclaimPolicy: Delete
  csi:
    controllerExpandSecretRef:
      name: csi-new
      namespace: d8-ceph-csi
    driver: rbd.csi.ceph.com
    fsType: ext4
    nodeStageSecretRef:
      name: csi-new
      namespace: d8-ceph-csi
    volumeAttributes:
      clusterID: 60f356ee-7c2d-4556-81be-c24b34a30b2a
      imageFeatures: layering
      imageName: csi-vol-f32fea79-d658-4ab1-967a-fb6e8f930dec
      journalPool: kube
      pool: kube
      storage.kubernetes.io/csiProvisionerIdentity: 1666697721019-8081-rbd.csi.ceph.com
  volumeHandle: 0001-0024-60f356ee-7c2d-4556-81be-c24b34a30b2a-0000000000000005-f32fea79-d658-4ab1-967a-fb6e8f930dec
  storageClassName: ceph-csi-rbd
  volumeMode: Filesystem
```

Создадим объект в кластере используя этот манифест.

На этом миграция завершена.
