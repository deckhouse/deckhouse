---
title: "Модуль local-path-provisioner: FAQ"
---

## Как настроить Prometheus на использование локального хранилища?

Применить CR `LocalPathProvisioner`:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

- `spec.nodeGroups` должен совпадать с нодгруппой prometheus'ов.
- `spec.path` - путь на узле где будут лежать данные.

Добавить в конфигурацию Deckhouse (configMap `d8-system/deckhouse`):
```yaml
prometheus: |
  longtermStorageClass: localpath-system
  storageClass: localpath-system
```

Дождаться переката Pod'ов Prometheus.

## Как изменить политику очистки?

На текущий момент политика удаления зашита в исходные коды и не может быть изменена [issue](https://github.com/deckhouse/deckhouse/issues/360)

## Почему папка не была удалена с сервера после очистки?

Если была выполнена команда по типу `kubectl delete -f demo.yml` она удалила все разом, включая `LocalPathProvisioner` который в свою очередь отвечает за фактическое удаление папки, другими словами просто некому выполнить `rm -rf /mnt/kubernetes/demo`

Для того что бы все сработало как надо, необходимо удалить pod'ы, следом pvc, уже затем модуль `LocalPathProvisioner` увидит эти изменения и очистит папки на сервере.
