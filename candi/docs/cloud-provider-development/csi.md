# Container Storage Interface

CSI driver управляет дисками подключенными к узлам кластера на основании StorageClass и PersistentVolumeClaim ресурсов.

Список Helm шаблонов, которые должны быть в репозитории Deckhouse cloud-provider:
- `templates/csi/controller.yaml` - генерирует ресурс Deployment для CSI controller и ресурс DaemonSet для CSI node plugin.
- `templates/csi/storageclass.yaml` - генерирует ресурсы StorageClass из данных содержащихся в ресурсе `Secret kube-system/d8-cloud-provider-discovery-data`, который создается сервисом `cloud-data-discoverer`.
- `templates/csi/csidriver.yaml` - генерирует ресурс CSIDriver для регистрации CSI driver в кластере.
- `templates/csi/rbac-for-us.yaml` - генерирует ресурсы RBAC для доступа CSI controller к API серверу.

Пример реализации CSI driver можно найти в отдельной ветке репозитория Deckhouse - [cloud-provider-sample](https://github.com/deckhouse/deckhouse/tree/cloud-provider-sample/ee/modules/030-cloud-provider-sample).
