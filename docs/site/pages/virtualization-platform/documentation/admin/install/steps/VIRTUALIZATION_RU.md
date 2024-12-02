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
sudo -i d8 k create -f - <<EOF
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
sudo -i d8 k get po -n d8-virtualization
```

  Пример вывода:
  {% snippetcut %}
  ```console
  user@master-0:~$ sudo -i d8 k get po -n d8-virtualization
  NAME                                         READY   STATUS    RESTARTS      AGE
  cdi-apiserver-858786896d-rsfjw               3/3     Running   0             10m
  cdi-deployment-6d9b646b5b-8dgmj              3/3     Running   0             10m
  cdi-operator-5fdc989d9f-zmk55                3/3     Running   0             10m
  dvcr-74dc9c94b-pczhx                         2/2     Running   0             10m
  virt-api-78d49dcbbf-qwggw                    3/3     Running   0             10m
  virt-controller-6f8fff445f-w866w             3/3     Running   0             10m
  virt-handler-g6l9h                           4/4     Running   0             10m
  virt-handler-t5fgb                           4/4     Running   0             10m
  virt-handler-ztj77                           4/4     Running   0             10m
  virt-operator-58dc5459d5-hpps8               3/3     Running   0             10m
  virtualization-api-5d69f55947-k6h9n          1/1     Running   0             10m
  virtualization-controller-69647d98c6-9rkht   3/3     Running   0             10m
  vm-route-forge-288z7                         1/1     Running   0             10m
  vm-route-forge-829wm                         1/1     Running   0             10m
  vm-route-forge-nq9xr                         1/1     Running   0             10m
  ```
  {% endsnippetcut %}
