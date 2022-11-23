## Миграции PersistentVolume's с in-tree rbd driver на csi driver (Ceph CSI)

Для упрощения миграции был написан скрипт [rbd-in-tree-to-ceph-csi-migration-helper.sh](../../tools/rbd-in-tree-to-ceph-csi-migration-helper.sh).
С его помощью можно мигрировать отдельный волюм (пару PVC и PV). Перед запуском В процессе миграции будет необходимо вручную выполнить команду в ceph-кластере для переименования rbd-образа.


Требования:
* Включен и настроен модуль ceph-csi.
* Pod использующий PersistentVolumeClaim отсутствует.


Последовательность действий:
1. Сохраняем манифесты pvc и pv которые будем мигрировать.
2. Создаем pvc и pv которые будут использоваться в качестве образца.
3. Переименовываем образ в ceph-кластере.
4. Модифицируем манифест исходного pvc и применяем его в кластере.
5. Модифицируем манифест исходного pv  и применяем его в кластере.

Исходные данные:
* `rbd` - storageСlass использующий старый драйвер (in-tree)
* `rbd-new` - storageСlass использующий новый драйвер (csi)
* `PersistentVolumeClaim` и `PersistentVolume` которые будем мигрировать:
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

2. Создадим PersistentVolumeClaim используя StorageClass соданный модулем ceph-csi, из которого автоматически будет создан PersistentVolume. Далее их будем использовать как доноров.

```yaml
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
  storageClassName: new-rbd
```


1. Поскольку Ceph CSI драйвер использует другой формат имени rbd-образа, переименуем его в ceph кластере:
    ```shell
rbd mv kube/kubernetes-dynamic-pvc-f32fea79-d658-4ab1-967a-fb6e8f930dec kube/csi-vol-f32fea79-d658-4ab1-967a-fb6e8f930dec
```
    * `kubernetes-dynamic-pvc-<uid>` - старый формат;
    * `csi-vol-<uid>` - новый формат.

4. Удалим исходные `PersistentVolumeClaim` и `PersistentVolume` из кластера:
    ```shell
kubectl -n default delete pvc data-test-0
kubectl delete pv pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
```
    Т.к. в предыдущем пункте мы переименовали rbd-образ в ceph-кластере, то удаление PersistentVolume не повлечет удаление образа.

5. Подготовим новый PersistentVolumeClaim используя исходный и созданный ранее sample.
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
  storageClassName: new-rbd
  volumeMode: Filesystem
  volumeName: pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
```

1. Создадим PersistentVolumeClaim в кластере использую полученный манифест. После создания PersistentVolumeClaim из него необходимо взять `.metadata.resourceVersion` и `.metadata.uid`. Их мы будем использовать при подготовке манифеста PersistentVolume.

2. Подготовим сохранённый PersistentVolume.
    * Удалим `.metadata.creationTimestamp`, `.metadata.resourceVersion`, `.metadata.uid`, `.status` и `.spec.rbd`.
    * Заменим `.spec.claimRef.resourceVersion`, `.spec.claimRef.uid`, `.spec.storageClassName` и `.metadata.annotations` и добавим `.spec.csi`. В качестве источника данных для `.spec.storageClassName` и `.metadata.annotations` используем ранее созданный PersistentVolume, а `.spec.claimRef.resourceVersion`, и `.spec.claimRef.uid` берём из только что созданного PersistentVolumeClaim data-test-0 (`.metadata.resourceVersion` и `.metadata.uid`). Секцию `.spec.csi` возьмем из созданного во втором пункте PersistentVolume:
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
    imageName: csi-vol-880ec27e-5b75-11ed-a252-fa163ee74632
    journalPool: kube
    pool: kube
    storage.kubernetes.io/csiProvisionerIdentity: 1666697721019-8081-rbd.csi.ceph.com
  volumeHandle: 0001-0024-60f356ee-7c2d-4556-81be-c24b34a30b2a-0000000000000005-880ec27e-5b75-11ed-a252-fa163ee74632
```

    В полях `imageName` и `volumeHandle` заменим uid (в данном примере `880ec27e-5b75-11ed-a252-fa163ee74632`) на uid волюма, который мигрируем.

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
  storageClassName: new-rbd
  volumeMode: Filesystem
```

1. Создадим PersistentVolume в кластере использую полученный манифест.

На этом миграция завершена и можно запускать Pod использующий волюм.
