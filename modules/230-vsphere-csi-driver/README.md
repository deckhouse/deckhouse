Модуль vsphere-csi-driver
=======

Модуль устанавливает [vsphere-csi-driver](https://github.com/kubernetes-sigs/vsphere-csi-driver) в кластер для получения возможности заказа дисков в vSphere.
Так же будут созданы указанные storage class'ы.

Данный модуль работает только в кластерах версии 1.14 и выше.

У пользователя, с помощью которого будет происходить взаимодействие с vSphere, должны быть такие [права](https://vmware.github.io/vsphere-storage-for-kubernetes/documentation/vcp-roles.html#dynamic-provisioning).

На данный момент мы не поддерживаем ни Multi-vCenter ни Multi-Datacenter инсталляции, хотя [драйвер и умеет](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v0.2.0/docs/deploying_ccm_and_csi_with_multi_dc_vc_aka_zones.md) – мы предполагаем, что кластер развернут в одном vCenter и у всех узлов есть доступ ко всем Datastore.

Важная информация об увеличении размера PVC
-----------------

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и vSphere API, перед увеличением PVC нужно заскейлить Deployment или StatefulSet в 0, после этого увеличить PVC, убедившись в успешном увеличении через `kubectl get pvc -o yaml` (в Status размер должен быть равен Spec, и не должно быть никаких conditions).

Конфигурация
------------

**Важно!** Перед тем, как приступить к настройке модуля, убедитесь, что у вас [включен `disk.EnableUUID` для всех машин](docs/disk_uuid.md).

Для включения данного модуля кластер Kubernetes должен быть не ниже 1.14 и необходимо задать параметр `virtualCenter` с настройками подключения к vSphere.

### Параметры

* `virtualCenter` — настройка виртуального датацентра для доступа к vSphere.
  * `address` — адрес (домен или IP-адрес), по которому доступен vCenter в vSphere;
  * `username` — имя пользователя для взаимодействия с vSphere;
  * `password` — пароль пользователя;
  * `datacenter` — датацентр в данном vCenter;
  * `port` — порт по которому доступен vSphere;
    * По-умолчанию `443`.
  * `insecure` — разрешить доступ к vSphere с самоподписанным сертификатом;
    * По-умолчанию `false`.
* `storageClasses` — список storageClass'ов, которые будут созданы:
  * `name` — постфикс имени storageClass'а (будет создан `vsphere-<name>`);
  * `datastore` — имя хранилища;
  * `default` — будет ли данный storageClass дефолтным в kubernetes.
    * По-умолчанию `false`.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/vsphere-csi-driver":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет настроено значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"vsphere-csi-driver"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфигурации

```yaml
vsphereCsiDriver: |
  virtualCenter:
    address: p-vc.corp.example.com
    username: VolumeProvisioner@corp.example.com
    password: p@$$word
    datacenter: X1
  storageClasses:
  - name: main
    datastore: LUN_101
    default: true
  - name: foo
    datastore: LUN_102
```
