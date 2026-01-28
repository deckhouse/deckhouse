## Дополнительная конфигурация кластера

На master-узле кластера создайте файл `additional_configuration.yml`:

{% capture includePath %}_includes/getting_started/dvp/bm/partials/config.ru.yml.other.ce.inc{% endcapture %}
{% include_file "{{ includePath }}" syntax="yaml" %}

После этого примените файл конфигурации, выполнив команду:

```console
sudo -i d8 k apply -f additional_configuration.yml
```

## Проверка работоспособности всех компонентов

### Проверка доступности узлов кластера

Выведите список всех узлов кластера, выполнив на master-узле следующую команду:

```bash
sudo -i d8 k get no
```

Убедитесь, что все узлы находятся в состоянии `Ready`. Пример корректного вывода:

```console
NAME     STATUS   ROLES           AGE   VERSION
master   Ready    control-plane   15m   v1.30.0
worker   Ready    <none>          12m   v1.30.0
```

### Проверка работоспособности хранилища NFS

1. Убедитесь, что модуль `csi-nfs` находится в состоянии `Ready`:

   ```bash
   sudo -i d8 k get module csi-nfs -w
   ```

1. Проверьте, что NFSStorageClass создан успешно:

   ```bash
   sudo -i d8 k get nfsstorageclass
   ```

1. Проверьте, что StorageClass установлен как используемый по умолчанию:

   ```bash
   sudo -i d8 k get storageclass
   ```

   В колонке `DEFAULT` у `nfs-storage-class` должна быть отметка.

### Проверка работоспособности модуля `virtualization`

Дождитесь, пока все поды модуля `virtualization` не перейдут в статус `Running`:

```bash
sudo -i d8 k get po -n d8-virtualization
```

Пример вывода:

```console
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

### Проверка доступа к кластеру DVP

Для доступа к веб-интерфейсу Deckhouse Virtualization Platform выполните следующие действия:

1. Откройте в браузере адрес `console.domain.my`;
1. Введите учетные данные администратора, которые были созданы на этапе настройки доступа.
1. Убедитесь, что веб-интерфейс открывается и отображается корректно.

Поздравляем, ваш кластер готов к работе. Вы успешно настроили:

- master-узел с развернутой Deckhouse Virtualization Platform;
- worker-узел для запуска рабочих нагрузок;
- NFS-хранилище для данных;
- модуль `virtualization` для создания виртуальных машин;
- Веб-интерфейс для управления платформой;
- Ingress-контроллер для доступа к веб-интерфейсу и к виртуальным машинам.

Далее рассмотрим дальнейшие возможности использования Deckhouse Virtualization Platform.
