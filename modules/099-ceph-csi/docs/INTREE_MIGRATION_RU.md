## Миграции PersistentVolume's с in-tree rbd driver на csi driver (ceph-csi)

Предполагается, что включен и настроен модуль ceph-csi, Pod использующий PersistentVolumeClaim отсутствует.


1. В качестве примера возьмем `PersistentVolumeClaim` и `PersistentVolume`:

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

    Сохраним манифесты, т.к. в дальнейшем их потребуетя удалить из кластера.


2. Создадим PersistentVolumeClaim использую StorageClass соданный модулем ceph-csi, из которого автоматически будет создан PersistentVolume. Они нужны для получения данных необходимых для миграции.

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


3. Переименуем rbd образ в ceph кластере:

    ```shell
rbd mv kube/kubernetes-dynamic-pvc-f32fea79-d658-4ab1-967a-fb6e8f930dec kube/csi-vol-f32fea79-d658-4ab1-967a-fb6e8f930dec
```

4. Удалим сохранённые PersistentVolumeClaim и PersistentVolume из кластера.

    ```shell
kubectl -n default delete pvc data-test-0
kubectl -n default delete pv pvc-cd6f7b26-d768-4cab-88a4-baca5b242cc5
```

    Т.к. в предыдущем пункте мы переименовали rbd-образ в ceph-кластере, то удаление PersistentVolume не повлечет удаление образа.

5. Подготовим сохранённый PersistentVolumeClaim.
    * Удалим `.metadata.creationTimestamp`, `.metadata.resourceVersion`, `.metadata.uid` и `.status`.
    * Заменим `.spec.storageClassName` и `.metadata.annotations`. В качестве источника данных для `.spec.storageClassName` и `.metadata.annotations` используем ранее созданный PersistentVolumeClaim sample.

    В итоге получится манифест:

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

6. Создадим PersistentVolumeClaim в кластере использую полученный манифест. После создания PersistentVolumeClaim из него необходимо взять `.metadata.resourceVersion` и `.metadata.uid`. Их мы будем использовать при подготовке манифеста PersistentVolume.

7. Подготовим сохранённый PersistentVolume.
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

8. Создадим PersistentVolume в кластере использую полученный манифест.

На этом миграция завершена и можно запускать Pod использующий волюм.
