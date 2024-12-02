---
title: "Настройка виртуализации"
permalink: ru/virtualization-platform/documentation/admin/install/steps/virtualization.html
lang: ru
---

## Настройка виртуализации

После настройки хранилища нужно включить модуль виртуализации. Включение и настройка производятся с помощью ресурса ModuleConfig virtualization.

В параметрах `spec` нужно установить:
- `enabled: true` — флаг для включения модуля;
- `settings.virtualMachineCIDRs` — подсети, IP-адреса из которых будут назначаться виртуальным машинам;
- `settings.dvcr.storage.persistentVolumeClaim.size` — размер дискового пространства под хранение образов виртуальных машин.

Пример конфигурации модуля virtualization:

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
    - 10.66.10.0/24
    - 10.66.20.0/24
    - 10.66.30.0/24
  version: 1
EOF
```

Дождитесь, что все поды модуля перешли в статус `Running`:

```shell
d8 k get po -n d8-system
```
