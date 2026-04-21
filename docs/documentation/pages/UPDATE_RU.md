---
title: "Руководство по обновлению Deckhouse Kubernetes Platform Certified Security Edition"
permalink: ru/update.html
lang: ru
---

<style>
.docs ol > li::marker {
  content: counters(list-item, ".") ". ";
}
</style>

В данном руководстве описан процесс обновления ПО «Deckhouse Kubernetes Platform Certified Security Edition» (далее — DKP CSE) с версии v1.67.4 до v1.73.1.

{% raw %}
## Минимальные требования к узлам кластера для обновления

Требования к аппаратной части:

- архитектура процессора x86-64;
- не менее 4 ядер CPU;
- не менее 12 ГБ RAM;
- не менее 50 ГБ дискового пространства.

Требования к программной части:

- ОС РЕД ОС (не ниже 7.3);
- ОС Astra Linux Special Edition (не ниже 1.7);
- ОС Альт не ниже 8 СП (релиз не ниже 10);
- Московская серверная операционная система (не ниже 15.5).

## С какой версии DKP CSE возможно обновление

Обновление возможно с версии DKP CSE 1.67.4.

## Действия перед обновлением

1. Загрузите предоставленное вам обновление в ваше хранилище образов контейнеров DKP CSE.

2. Перед обновлением убедитесь в отсутствии в кластере алертов, кроме `D8DeckhouseIsNotOnReleaseChannel`.

3. Убедитесь, что в очередях DKP CSE нет ошибок.

   ```bash
   d8 system queue list
   ```

   Пример вывода команды пустых очередей без ошибок:

   ```bash
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

4. При наличии NodeGroupConfiguration `sysctl-tune-fstec` добавьте аннотации и лейбл.

   Проверка наличия NodeGroupConfiguration:

   ```bash
   d8 k get ngc | grep sysctl-tune-fstec
   ```

   Пример вывода:

   ```bash
   sysctl-tune-fstec          100      ["*"]        ["*"]
   ```

   Команды для добавления аннотаций и лейбла:

   ```bash
   d8 k annotate ngc sysctl-tune-fstec meta.helm.sh/release-namespace=d8-system
   d8 k annotate ngc sysctl-tune-fstec meta.helm.sh/release-name=node-manager
   d8 k label ngc sysctl-tune-fstec app.kubernetes.io/managed-by=Helm
   ```

5. Если вы используете `local-path-provisioner` в качестве драйвера хранилища для ваших StatefulSet или модулей платформы:

   1. В случае, если включен модуль `loki`, потребуется совершить ряд действий для корректного обновления платформы.
 
   2. Проверьте состояние модуля `loki`, выполнив команду:
 
      ```bash
      d8 k get module | grep loki
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Downloaded   False     False
         ```

         Дальнейших действий с модулем `loki` не требуется.

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Ready        True      True
         ```

   3. Убедитесь, что в хранилище достаточно свободного дискового пространства для расширения хранилища данных модуля loki (не менее 50 ГБ).

   4. Измените размер PVC для loki:

      ```bash
      d8 k -n d8-monitoring edit pvc storage-loki-0
      ```
   
      Установите значение параметра `spec.resources.requests.storage: 50Gi`,
      как в примере:
   
      ```yaml
      ...
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 50Gi
      ...
      ```

   5. Измените размер PV, которую использует модуль loki:

      1. Получите имя PV для изменения, выполнив команду:

         ```bash
         d8 k get pv | grep loki
         ```

         Пример вывода:
    
         ```bash
         pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd   2Gi        RWO            Delete           Bound    d8-monitoring/storage-loki-0          localpath           <unset>                          3m40s
         ```
    
         Искомый PV должен находиться в пространстве имён `d8-monitoring`.
         В данном случае название PV для изменения будет `pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd`.

      2. Измените размер PV с именем, полученным из предыдущей команды, выполнив команду:

         ```bash
         d8 k edit pv pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd
         ```
   
         Установите значение параметра `spec.capacity.storage: 50Gi`,
         как в примере:
   
         ```yaml
         ...
         spec:
           accessModes:
           - ReadWriteOnce
           capacity:
             storage: 50Gi
           claimRef:
         ...
         ```

   6. Убедитесь, что PV для модуля loki имеет новый размер, выполнив команду:

      ```bash
      d8 k get pv | grep loki
      ```

      Пример вывода:

      ```bash
      pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd   50Gi       RWO            Delete           Bound    d8-monitoring/storage-loki-0          localpath           <unset>                          11m
      ```

   7. Удалите StatefulSet `loki`, выполнив команду:

      ```bash
      d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-monitoring delete sts loki
      ```

      Для проверки успешного завершения удаления выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Вывод команды должен быть пустым.

   8. Добавьте в ModuleConfig `loki` параметр `spec.settings.diskSizeGigabytes: 50`, выполнив команду:
  
      ```bash
      d8 k edit mc loki
      ```

      Пример конфигурации:

      ```yaml
      ...
      spec:
        enabled: true
        settings:
          diskSizeGigabytes: 50
          retentionPeriodHours: 24
          storageClass: localpath
        version: 1
      ...
      ```

   9. Дождитесь перехода пода loki в состояние `Running`:

      Выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Пример успешного вывода:

      ```bash
      loki-0                   2/2     Running   0             5m28s
      ```

6. Если вы используете `csi-nfs` в качестве драйвера хранилища для ваших StatefulSet или модулей платформы:

   1. Для корректной работы модуля `csi-nfs` после обновления требуется включение модуля `snapshot-controller`.
      Проверьте состояние модуля `snapshot-controller`, выполнив команду:

      ```bash
      d8 k get modules | grep snapshot-controller
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         snapshot-controller                   37       Embedded   Downloaded   False     False
         ```
    
         Для включения модуля `snapshot-controller` выполните команду:

         ```bash
         d8 system module enable snapshot-controller
         ```

         Пример вывода команды:

         ```bash
         Module snapshot-controller enabled
         ```

         Дождитесь разбора очередей DKP CSE.

         Проверка очередей DKP CSE:

         ```bash
         d8 system queue list
         ```

         Пример вывода пустых очередей без ошибок:

         ```bash
         Summary:
         - 'main' queue: empty.
         - 103 other queues (0 active, 103 empty): 0 tasks.
         - no tasks to handle.
         ```

         Проверьте состояние модуля `snapshot-controller`, выполнив команду:

         ```bash
         d8 k get modules | grep snapshot-controller
         ```

         Пример вывода при включенном модуле:

         ```bash
         snapshot-controller                   37       Embedded   Ready        True      True
         ```

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         snapshot-controller                   37       Embedded   Ready        True      True
         ```

         Дальнейших действий с модулем `snapshot-controller` не требуется. 

   2. В случае, если включен модуль `loki`, потребуется совершить ряд действий для корректного обновления платформы.

   3. Проверьте состояние модуля `loki`, выполнив команду:

      ```bash
      d8 k get module | grep loki
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Downloaded   False     False
         ```

         Дальнейших действий с модулем `loki` не требуется.

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Ready        True      True
         ```
    
   4. Убедитесь, что в хранилище достаточно свободного дискового пространства для расширения хранилища данных модуля loki (не менее 50 ГБ).

   5. Измените размер PVC для loki:

      ```bash
      d8 k -n d8-monitoring edit pvc storage-loki-0
      ```

      Установите значение параметра `spec.resources.requests.storage: 50Gi`,
      как в примере:

      ```yaml
      ...
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 50Gi
      ...
      ```

   6. Убедитесь, что PV для модуля loki имеет новый размер, выполнив команду:

      ```bash
      d8 k get pv | grep loki
      ```

      Пример вывода:

      ```bash
      pvc-4898b2a1-c432-472d-9d90-46d117838f39   50Gi       RWO            Delete           Bound    d8-monitoring/storage-loki-0          nfs-storage-class   <unset>                          23m
      ```

   7. Удалите StatefulSet `loki`

      ```bash
      d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-monitoring delete sts loki
      ```

      Для проверки успешного завершения удаления выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Вывод команды должен быть пустым.

   8. Добавьте в ModuleConfig `loki` параметр `spec.settings.diskSizeGigabytes: 50`
      Пример конфигурации:

      ```yaml
      ...
      spec:
        enabled: true
        settings:
          diskSizeGigabytes: 50
          retentionPeriodHours: 24
          storageClass: nfs-storage-class
        version: 1
      ...
      ```

   9. Дождитесь перехода пода loki в состояние `Running`:

      Выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Пример успешного вывода:

      ```bash
      loki-0                   2/2     Running   0             5m28s
      ```

7. Если вы используете `sds-local-volume` в качестве драйвера хранилища для ваших StatefulSet или модулей платформы:

   1. Для корректной работы модуля `sds-local-volume` после обновления требуется включение модуля `snapshot-controller`.
      Проверьте состояние модуля `snapshot-controller`, выполнив команду:

      ```bash
      d8 k get modules | grep snapshot-controller
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         snapshot-controller                   37       Embedded   Downloaded   False     False
         ```
    
         Для включения модуля `snapshot-controller` выполните команду:

         ```bash
         d8 system module enable snapshot-controller
         ```

         Пример вывода команды:

         ```bash
         Module snapshot-controller enabled
         ```

         Дождитесь разбора очередей DKP CSE.

         Проверка очередей DKP CSE:

         ```bash
         d8 system queue list
         ```

         Пример вывода команды пустых очередей без ошибок:

         ```bash
         Summary:
         - 'main' queue: empty.
         - 103 other queues (0 active, 103 empty): 0 tasks.
         - no tasks to handle.
         ```

         Проверьте состояние модуля `snapshot-controller`, выполнив команду:

         ```bash
         d8 k get modules | grep snapshot-controller
         ```

         Пример вывода при включенном модуле:

         ```bash
         snapshot-controller                   37       Embedded   Ready        True      True
         ```

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         snapshot-controller                   37       Embedded   Ready        True      True
         ```

         Дальнейших действий с модулем `snapshot-controller` не требуется. 

   2. В случае, если включен модуль `loki`, потребуется совершить ряд действий для корректного обновления платформы.

   3. Проверьте состояние модуля `loki`, выполнив команду:

      ```bash
      d8 k get module | grep loki
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Downloaded   False     False
         ```

         Дальнейших действий с модулем `loki` не требуется.

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Ready        True      True
         ```

   4. Убедитесь, что в хранилище достаточно свободного дискового пространства для расширения хранилища данных модуля loki (не менее 50 ГБ).

   5. Измените размер PVC для loki:

      ```bash
      d8 k -n d8-monitoring edit pvc storage-loki-0
      ```

      Установите значение параметра `spec.resources.requests.storage: 50Gi`,
      как в примере:

      ```yaml
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 50Gi
      ```

   6. Убедитесь, что PV для модуля loki имеет новый размер, выполнив команду:

      ```bash
      d8 k get pv | grep loki
      ```

      Пример вывода:

      ```bash
      pvc-ca0676ce-4ad2-41c6-bab4-6fc034e98ef2   50Gi       RWO            Delete           Bound    d8-monitoring/storage-loki-0          local-storage-class   <unset>                          23m
      ```

   7. Удалите StatefulSet `loki`:

      ```bash
      d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-monitoring delete sts loki
      ```

      Для проверки успешного завершения удаления выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Вывод команды должен быть пустым.

   8. Добавьте в ModuleConfig `loki` параметр `spec.settings.diskSizeGigabytes: 50`
      Пример конфигурации:

      ```yaml
      ...
      spec:
        enabled: true
        settings:
          diskSizeGigabytes: 50
          retentionPeriodHours: 24
          storageClass: local-storage-class
        version: 1
      ...
      ```

   9. Дождитесь перехода пода loki в состояние `Running`

      Выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Пример успешного вывода:

      ```bash
      loki-0                   2/2     Running   0             5m28s
      ```

8. Если вы используете `csi-ceph` в качестве драйвера хранилища для ваших StatefulSet или модулей платформы:

   1. Для корректной работы модуля `csi-ceph` после обновления требуется включение модуля `snapshot-controller`.
      Проверьте состояние модуля `snapshot-controller`, выполнив команду:

      ```bash
      d8 k get modules | grep snapshot-controller
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         snapshot-controller                   37       Embedded   Downloaded   False     False
         ```
    
         Для включения модуля `snapshot-controller` выполните команду:

         ```bash
         d8 system module enable snapshot-controller
         ```

         Пример вывода команды:

         ```bash
         Module snapshot-controller enabled
         ```

         Дождитесь разбора очередей DKP CSE.

         Проверка очередей DKP CSE:

         ```bash
         d8 system queue list
         ```

         Пример вывода команды пустых очередей без ошибок:

         ```bash
         Summary:
         - 'main' queue: empty.
         - 103 other queues (0 active, 103 empty): 0 tasks.
         - no tasks to handle.
         ```

         Проверьте состояние модуля `snapshot-controller`, выполнив команду:

         ```bash
         d8 k get modules | grep snapshot-controller
         ```

         Пример вывода при включенном модуле:

         ```bash
         snapshot-controller                   37       Embedded   Ready        True      True
         ```

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         snapshot-controller                   37       Embedded   Ready        True      True
         ```

         Дальнейших действий с модулем `snapshot-controller` не требуется. 

   2. В случае, если включен модуль `loki`, потребуется совершить ряд действий для корректного обновления платформы.

   3. Проверьте состояние модуля `loki`, выполнив команду:

      ```bash
      d8 k get module | grep loki
      ```

      1. В случае выключенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Downloaded   False     False
         ```

         Дальнейших действий с модулем `loki` не требуется.

      2. В случае включенного модуля пример вывода будет следующим:

         ```bash
         loki           462      Embedded   Ready        True      True
         ```
  
   4. Убедитесь, что в хранилище достаточно свободного дискового пространства для расширения хранилища данных модуля loki (не менее 50 ГБ).

   5. Измените размер PVC для loki:

      ```bash
      d8 k -n d8-monitoring edit pvc storage-loki-0
      ```

      Установите значение параметра `spec.resources.requests.storage: 50Gi`,
      как в примере:

      ```yaml
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 50Gi
      ```

   6. Убедитесь, что PV для модуля `loki` имеет новый размер, выполнив команду:

      ```bash
      d8 k get pv | grep loki
      ```

      Пример вывода:

      ```bash
      pvc-86c93d6a-57be-4871-a15b-101c282a0019   50Gi       RWO            Delete           Bound    d8-monitoring/storage-loki-0          ceph-rbd-sc         <unset>                          3m2s
      ```

   7. Удалите StatefulSet `loki`

      ```bash
      d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-monitoring delete sts loki
      ```

      Для проверки успешного завершения удаления выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Вывод команды должен быть пустым.

   8. Добавьте в ModuleConfig `loki` параметр `spec.settings.diskSizeGigabytes: 50`
      Пример конфигурации:

      ```yaml
      ...
      spec:
        enabled: true
        settings:
          diskSizeGigabytes: 50
          retentionPeriodHours: 24
          storageClass: ceph-rbd-sc
        version: 1
      ...
      ```

   9. Дождитесь перехода пода loki в состояние `Running`.

      Выполните команду:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Пример успешного вывода:

      ```bash
      loki-0                   2/2     Running   0             5m28s
      ```


9. Переведите обновления версии Kubernetes в кластере в ручной режим.

   1. Проверьте версию Kubernetes внутри ресурса `ClusterConfiguration`, выполнив команду:

      ```bash
      d8 system edit cluster-configuration
      ```

      - В случае, если значение параметра `kubernetesVersion:` равно `"1.29"`, действий не требуется.

      - В случае, если значение параметра `kubernetesVersion:` равно `"1.27"`, установите значение параметра `kubernetesVersion:` равным `"1.29"` и дождитесь обновления версии Kubernetes на всех узлах и разбора очередей DKP CSE.

      - В случае, если значение параметра `kubernetesVersion:` равно `Automatic`, установите значение параметра `kubernetesVersion:` равным `"1.29"`.

   2. Проверка очередей DKP CSE:

      ```bash
      d8 system queue list
      ```

      Пример вывода команды пустых очередей без ошибок:

      ```bash
      Summary:
      - 'main' queue: empty.
      - 103 other queues (0 active, 103 empty): 0 tasks.
      - no tasks to handle.
      ```

   3. Проверка статуса обновления всех узлов в кластере:

      ```bash
      d8 k get ng
      ```

      Пример вывода команды с успешно обновлёнными узлами:

      ```bash
      NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
      master   Static   3       3       3                                                               34h   True
      system   Static   2       2       2                                                               34h   True
      worker   Static   3       3       3                                                               34h   True
      ```
      {: .nowrap-default }

      Количество узлов `UPTODATE` должно быть равно количеству `NODES` и `READY`.

10. Убедитесь в отсутствии релизного канала в ModuleConfig `deckhouse`.

    1. Проверка актуального ModuleConfig:
 
       ```bash
       d8 k get mc deckhouse -o yaml
       ```
 
       Пример вывода:
 
       ```yaml
       apiVersion: deckhouse.io/v1alpha1
       kind: ModuleConfig
       metadata:
         creationTimestamp: "<some-timestamp>"
         finalizers:
         - modules.deckhouse.io/module-config
         generation: 1
         name: deckhouse
         resourceVersion: "<some-version>"
         uid: <some-uid>
       spec:
         enabled: true
         settings:
           bundle: Default
           logLevel: Info
         version: 1
       status:
         message: ""
         version: "1"
       ```
 
       При наличии поля `spec.settings.releaseChannel` удалите его, выполнив команду:
 
       ```bash
       kubectl patch mc deckhouse --type='merge' -p='{"spec":{"settings":{"releaseChannel": null}}}'
       ```
 
       Результат выполнения команды:
 
       ```bash
       moduleconfig.deckhouse.io/deckhouse patched
       ```

## Обновление версии платформы

1. Примените NodeGroupConfiguration `bashible-migrations-002.sh` следующего вида:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: bashible-migrations-002.sh
   spec:
     bundles: ['*']
     content: |
       if ! declare -F bb-d8-node-name >/dev/null; then
         echo "Function 'bb-d8-node-name' NOT found, restarting 'bashible' service..."
         sleep 30
         systemctl restart bashible.service
         echo "Service 'bashible' restarted."
       fi
     nodeGroups:
     - '*'
     weight: 002
   ```

2. Поменяйте образ в Deployment `deckhouse`.

   ```bash
   d8 k --as=system:serviceaccount:d8-system:deckhouse set image deployment -n d8-system deckhouse deckhouse=<your-cse-registry>/deckhouse/cse:v1.73.1
   ```

3. Дождитесь разбора очередей DKP CSE и полного обновления всех узлов в кластере.

   1. Проверка очередей DKP CSE:

      ```bash
      d8 system queue list
      ```

      Пример вывода команды пустых очередей без ошибок:

      ```bash
      Summary:
      - 'main' queue: empty.
      - 103 other queues (0 active, 103 empty): 0 tasks.
      - no tasks to handle.
      ```

   2. Проверка статуса обновления всех узлов в кластере:

      ```bash
      d8 k get ng
      ```

      Пример вывода команды с успешно обновлёнными узлами:

      ```bash
      NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
      master   Static   3       3       3                                                               34h   True
      system   Static   2       2       2                                                               34h   True
      worker   Static   3       3       3                                                               34h   True
      ```
      {: .nowrap-default }

      Количество узлов `UPTODATE` должно быть равно количеству `NODES` и `READY`.

4. Обращайте внимание на появление алертов вида `NodeRequiresDisruptionApprovalForUpdate`. В случае возникновения следуйте инструкции в алерте.

5. Обновите версию `control-plane-manager`, изменив поле `spec.version` в ModuleConfig `control-plane-manager` с **1** на **2**.

   ```bash
   d8 k edit mc control-plane-manager
   ```

   Пример ModuleConfig с валидной версией:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
   spec:
     settings:
     ...
     version: 2
   ```

6. Обновите версию Kubernetes до 1.31.

   Выполните команду:

   ```bash
   d8 system edit cluster-configuration
   ```

   В ресурсе `ClusterConfiguration` измените версию Kubernetes на `Automatic`, дождитесь обновления версии Kubernetes на всех узлах.

   - Узлы с политикой применения деструктивных изменений `Auto` будут обновлены в автоматическом режиме;

   - Узлы с политикой применения деструктивных изменений `Manual` потребуют ручного применения изменений, о чем будет свидетельствовать алерт вида `NodeRequiresDisruptionApprovalForUpdate`. Для применения изменений следуйте инструкции в алерте.

   Команда для получения версии Kubernetes на узлах:

   ```bash
   d8 k get no -o custom-columns=NAME:.metadata.name,VERSION:.status.nodeInfo.kubeletVersion
   ```

   Пример вывода:

   ```bash
   NAME                                         VERSION
   example-cse-master-0.ru-central1.internal   v1.31.13
   example-cse-master-1.ru-central1.internal   v1.31.13
   example-cse-master-2.ru-central1.internal   v1.31.13
   example-cse-system-0.ru-central1.internal   v1.31.13
   example-cse-system-1.ru-central1.internal   v1.31.13
   example-cse-worker-0.ru-central1.internal   v1.31.13
   example-cse-worker-1.ru-central1.internal   v1.31.13
   example-cse-worker-2.ru-central1.internal   v1.31.13
   ```

   Проверка статуса обновления узлов в каждой группе:

   ```bash
   d8 k get ng
   ```

   Пример вывода команды с успешно обновлёнными узлами:

   ```bash
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   master   Static   3       3       3                                                               34h   True
   system   Static   2       2       2                                                               34h   True
   worker   Static   3       3       3                                                               34h   True
   ```
   {: .nowrap-default }

   Количество узлов `UPTODATE` должно быть равно количеству `NODES` и `READY`.

7. Удалите NodeGroupConfiguration `bashible-migrations-002.sh`:

   ```bash
   d8 k delete ngc bashible-migrations-002.sh
   ```

8. Установите канал обновлений в ModuleConfig `deckhouse`.

   Выполните команду:

   ```bash
   d8 k edit mc deckhouse
   ```

   В открывшемся окне установите параметр `spec.settings.releaseChannel: LTS`.

   Пример ModuleConfig с заданным параметром:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     enabled: true
     settings:
       bundle: Default
       logLevel: Info
       releaseChannel: LTS
     version: 1
   ```

## Включение контроля целостности

1. Миграция данных etcd для включения контроля целостности.

   1. Переведите режим работы контроля целостности на **Migrate**.

      Пример ModuleConfig `control-plane-manager` с режимом `Migrate`:

      ```yaml
      apiVersion: deckhouse.io/v1alpha1
      kind: ModuleConfig
      metadata:
        name: control-plane-manager
      spec:
        settings:
          apiserver:
            signature: Migrate
      ...
      ```

   2. Дождитесь очистки очередей DKP CSE.

      ```bash
      d8 system queue list 
      ```

      Пример вывода команды пустых очередей без ошибок:

      ```bash
      Summary:
      - 'main' queue: empty.
      - 103 other queues (0 active, 103 empty): 0 tasks.
      - no tasks to handle.
      ```

   3. Осуществите миграцию данных.

      Запустите утилиту от пользователя root с master-узла кластера со следующими аргументами:

      ```bash
      d8 tools sig-migrate
      ```

      **Рекомендуется запуск в эмуляторе терминала по типу screen или tmux. В случае отсутствия данных утилит на master-узле необходима их установка.**
      По окончании выполнения, если на какие-либо объекты не удалось установить аннотацию, команда автоматически выведет предупреждающее сообщение со следующей информацией:

      - Количество объектов, для которых не удалось произвести миграцию;
      - Пути к файлам с логами ошибок;
      - Инструкции по расследованию и запуску повторной попытки.

      Пример вывода при наличии ошибок:

      ```console
      ⚠️ Migration completed with 5 failed object(s).

      Some objects could not be annotated. Please check the error details:

      - Error log file: `/tmp/failed_errors.txt`
      - Failed objects list: `/tmp/failed_annotations.txt`

      To investigate the issues:
      1. Review the error log file to understand why objects failed
      2. Check permissions and resource availability
      3. Retry migration for failed objects only using:

         d8 tools sig-migrate --retry
      ```

      Для повторной установки аннотаций только на объекты, для которых не удалось произвести миграцию, используйте флаг `--retry`:

      ```bash
      d8 tools sig-migrate --retry
      ```

      Повторять процедуру миграции следует до тех пор, пока утилита не сообщит, что больше нет объектов, для которых не удалось установить аннотацию. А также до прекращения алерта `D8SignatureErrorsDetected`.

      В случае многократного (больше 10 раз) завершения с ошибкой **повторной** установки аннотаций обратитесь в техническую поддержку

   4. Переведите режим работы контроля целостности на **Enforce**.

      Пример ModuleConfig `control-plane-manager` с режимом `Enforce`:

      ```yaml
      apiVersion: deckhouse.io/v1alpha1
      kind: ModuleConfig
      metadata:
        name: control-plane-manager
      spec:
        settings:
          apiserver:
            signature: Enforce
      ...
      ```

   5. Дождитесь очистки очередей DKP CSE:

      ```bash
      d8 system queue list 
      ```

      Пример вывода команды пустых очередей без ошибок:

      ```bash
      Summary:
      - 'main' queue: empty.
      - 103 other queues (0 active, 103 empty): 0 tasks.
      - no tasks to handle.
      ```

## Обновление версии containerd

{% endraw %}
{% alert level="danger" %}
Не обновляйте версию containerd до v2, если в кластере используется модуль csi-ceph. Пропустите этот этап.
{% endalert %}
{% raw %}

Начиная с версии 1.73 ПО «Deckhouse Kubernetes Platform Certified Security Edition» (DKP CSE) доступна возможность использовать среду исполнения контейнеров обновлённой версии — containerd v2. Containerd v2 можно использовать как основную среду исполнения контейнеров на уровне всего кластера или для отдельных групп узлов. Использование containerd v2 обеспечивает более гибкое управление ресурсами и лучшую безопасность за счёт cgroups v2 и контроля целостности системных компонентов.

Требования к узлам для миграции на containerd v2:

- поддержка CgroupsV2;
- ядро Linux версии 5.8 и новее;
- systemd версии 244 и новее;
- поддержка модуля ядра erofs;
- отсутствие кастомных конфигураций в `/etc/containerd/conf.d`.

Также DKP CSE выполняет дополнительную проверку используемой версии ядра Linux на наличие известных уязвимостей. При обнаружении уязвимой версии ядра использование containerd v2 на узле запрещается, даже если все остальные требования выполнены.

Например, в РЕД ОС могут использоваться версии ядра Linux из приведённых ниже диапазонов, которые приводят к автоматическому ограничению использования containerd v2:

- ядра ветки 6.12.x с версиями 6.12.0–6.12.28 включительно.
  Исправление доступно начиная с версии 6.12.29;

- ядра ветки 6.14.x с версиями 6.14.0–6.14.6 включительно.
  Исправление доступно начиная с версии 6.14.7.

При несоответствии любому из требований для миграции, DKP CSE добавляет на узел лейбл `node.deckhouse.io/containerd-v2-unsupported`. Если на узле есть кастомные конфигурации в `/etc/containerd/conf.d`, на него добавляется лейбл `node.deckhouse.io/containerd-config=custom`.

При наличии одного из этих лейблов смена параметра spec.cri.type для группы узлов будет недоступна. Узлы, которые не подходят под условия миграции можно посмотреть с помощью следующих команд:

```bash
d8 k get node -l node.deckhouse.io/containerd-v2-unsupported
d8 k get node -l node.deckhouse.io/containerd-config=custom
```

Также администратор может проверить конкретный узел на соответствие требованиям с помощью команд:

```bash
uname -r | cut -d- -f1
stat -f -c %T /sys/fs/cgroup
systemctl --version | awk 'NR==1{print $2}'
modprobe -qn erofs && echo "TRUE" || echo "FALSE"
ls -l /etc/containerd/conf.d
```

Включение containerd v2 возможно двумя способами:

1. Для всего кластера.
2. Для конкретной группы узлов.

   Для включения containerd v2 для всего кластера укажите значение `ContainerdV2` в параметре `defaultCRI` ресурса `ClusterConfiguration`. Это значение будет применяться ко всем NodeGroup, в которых явно не указан `spec.cri.type`.

   ```bash
   d8 system edit cluster-configuration
   ```

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   ...
   defaultCRI: ContainerdV2
   ...
   ```

   Для включения containerd v2 для конкретной группы узлов укажите `ContainerdV2` в параметре `spec.cri.type` в объекте `NodeGroup`.
   Рассмотрим на примере группы узлов `worker`.

   ```bash
   d8 k edit ng worker
   ```

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     cri:
       type: ContainerdV2
   ...
   ```

   При переходе на containerd v2 DKP CSE начнет поочерёдное обновление узлов. Обновление узла приводит к прерыванию работы размещенной на нем нагрузки (disruptive-обновление). На процесс обновления узла влияют параметры применения disruptive-обновлений группы узлов (spec.disruptions.approvalMode NodeGroup).

   - Узлы с политикой применения disruptive-обновлений `Auto` будут обновлены в автоматическом режиме

   - Узлы с политикой применения disruptive-обновлений `Manual` потребуют ручного применения изменений, о чем будет свидетельствовать алерт вида `NodeRequiresDisruptionApprovalForUpdate`. Для применения изменений следуйте инструкции в алерте

## Особенности обновления в Astra Linux

Для проверки, включён ли режим ЗПС, выполните команды:

```bash
astra-digsig-control is-enabled
cat /sys/digsig/elf_mode
```

Для включения режима ЗПС на серверах под управлением Astra Linux используется ресурс NodeGroupConfiguration. В результате применения конфигурации:

- Проверяется, что модуль digsig_verif загружен в систему.
- Добавляются публичные ключи,используемые для валидации бинарных файлов компонентов DKP CSE.
- Включается проверка подписи бинарных файлов.
- Выполняется обновление образов initramfs для всех установленных ядер в системе.
- Выполняется перезагрузка системы.

Для применения конфигурации необходимо создать файл astra-zps-ngc.yaml и выполнить команду `kubectl apply -f astra-zps-ngc.yaml`.

Пример файла `astra-zps-ngc.yaml`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: astra-zps.sh
spec:
  weight: 98
  # Здесь вы можете указать конкретные группы узлов (имена NodeGroup)
  nodeGroups: [ "*" ]
  bundles: [ "astra" ]
  content: |
    # Copyright 2025 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    modeswitch="$(astra-modeswitch get 2>/dev/null)"
    case "$modeswitch" in
      0|1|2) ;;
      *) modeswitch=0 ;;
    esac

    if (( modeswitch < 1 )); then
      bb-log-info "Astra modeswitch level $modeswitch detected, skip digital signature provisioning"
      exit 1
    fi

    if ! grep -qw digsig_verif /proc/modules; then
      bb-log-warning "Module digsig_verif is not loaded"
      exit 1
    fi

    keys_dir="/etc/digsig/keys"
    digsig_conf="/etc/digsig/digsig_initramfs.conf"
    digsig_mode="1"

    mkdir -p "$keys_dir"
    mkdir -p "$(dirname "$digsig_conf")"

    _digsig_rebuild_initramfs() {
      bb-log-info "Rebuilding initramfs to refresh digital signature keys"
      update-initramfs -uk all
      bb-flag-set reboot
    }

    bb-event-on 'digsig-initramfs-update' '_digsig_rebuild_initramfs'

    is_enabled_zps() {
      astra-digsig-control is-enabled >/dev/null 2>&1
    }

    sync_key() {
      local filename="$1"
      local payload="$2"
      local target="$keys_dir/$filename"
      local tmp_file

      tmp_file="$(bb-tmp-file)"
      printf '%s' "$payload" | base64 -d > "$tmp_file"
      bb-sync-file "$target" "$tmp_file" digsig-initramfs-update
      chmod 600 "$target"
      rm -f "$tmp_file"
    }

    sync_digsig_conf() {
      if ! is_enabled_zps; then
        if ! astra-digsig-control enable; then
          bb-log-error "Failed to enable Astra digital signature control"
          exit 1
        fi
        bb-flag-set reboot
      fi
    }
    mkdir -p "$keys_dir/flant"
    
    sync_key "flant/flant-2025.gpg" mN4EaPngniMBAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQxAAMHAP9fv/SYqpOM5zm44CL7r+9AVj9uajRy/CpRTAzp2uI7fgACAgD8COKooOZRR9S9YxYDDhbRnIXJfwqcomcSK5arvOp+j8gAAQEBAIAAAAAAAAAAAAAAAAAAAAFQ/ooYkpdhVMWc/Bk6zPWzAP9Ict4rRYNZqoxJTYx1HHRNP3DYIpj7Z06CiD3ligMevgD+JEvcELVEzaAJpaESTiWhZRQWLSHwCGvkGXqFsL636G8AAQG0QdCQ0J4gItCk0LvQsNC90YIiICjQl9Cf0KEsIERIIHByb2R1Y3Rpb24ga2V5KSA8c2VjdXJpdHlAZmxhbnQucnU+iJAEEyMMADgWIQSP/y49Jif7FuB0hfMVqO+CTcPnQQUCaPngngIbAwULCQgHAgYVCgkICwIEFgIDAQIeAQIXgAAKCRAVqO+CTcPnQdnrAP0TrjvMKSqoNCYkJumjd744RYUT7g/NZ5dnXukQet0wbAD/a7/aCKtLkL75878crt8E+CMnsEgGrD39RkeZNZyT7UqIdQQQIwwAHRYhBPq2qyAJ6YwXAMD+S7IL0fENyY9ZBQJo+eDBAAoJELIL0fENyY9ZsA8A/jlimDJzpBDvs2s1saElUBdQ77c2o0G6Y7TA9MSD2uGEAP9SSgNP2OBGXBg7ztG28iO6BBmuBv2Ut3nQ648AawoLfg==
    sync_key "flant-2025-root.gpg" mN4EaD7ryyMBAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQxAAMHAP9fv/SYqpOM5zm44CL7r+9AVj9uajRy/CpRTAzp2uI7fgACAgD8COKooOZRR9S9YxYDDhbRnIXJfwqcomcSK5arvOp+j8gAAQEBAIAAAAAAAAAAAAAAAAAAAAFQ/ooYkpdhVMWc/Bk6zPWzAP9mAxC2f4hBFjBuYwaoFNziI75LPCXLMgUpNmObeXFHJQD9EBCPqnvFso5zQiB+4BfKjFWZoSH6V4PBa/JINtpYPxcAAQG0NNCQ0J4gwqvQpNC70LDQvdGCwrsgKGtleSBmb3Igc2lnbmluZykgPDc3N0BmbGFudC5ydT6IkAQTIwwAOBYhBPq2qyAJ6YwXAMD+S7IL0fENyY9ZBQJoPuvLAhsDBQsJCAcCBhUICQoLAgQWAgMBAh4BAheAAAoJELIL0fENyY9Z9BEA/0tB8rXfO2aLlpMLmyfPLDGvkURqHo+zJ6XRj9/aH+RpAP9e0QWrin5NI4pHPSOAuEx3pBT97nLNG3KOuDRdLLQrz4h1BBAjDAAdFiEEoS17W6/OQNh/1B2vyC1J/DZ1tvoFAmg+68wACgkQyC1J/DZ1tvqEAwD7B3xlPHa+pONWB1kxVj9+SZY+08phl3KTKW20Coq6EBAA/RCgTmuFPZDkQ+yRtK59TtefIzdTQq94ifhoxiSKsww7

    sync_digsig_conf
```

Для ручного отключения режима ЗПС выполните на сервере следующие команды:

```bash
sed -i '/^DIGSIG_ELF_MODE=/d' /etc/digsig/digsig_initramfs.conf && echo 'DIGSIG_ELF_MODE=0' | tee -a /etc/digsig/digsig_initramfs.conf
 update-initramfs -u -k all
 systemctl reboot
```

В качестве альтернативного варианта, для отключения режима ЗПС можно использовать утилиту `astra-digsig-control`:

```bash
astra-digsig-control disable
```

## Как понять, что обновление прошло успешно

1. В очередях DKP CSE нет заданий.

   Для проверки выполните команду:

   ```bash
   d8 system queue list 
   ```

   Пример вывода команды пустых очередей без ошибок:

   ```bash
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

2. Все узлы в кластере обновлены.

   Для проверки выполните команду:

   ```bash
   d8 k get ng
   ```

   Пример вывода команды с успешно обновлёнными узлами:

   ```bash
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   master   Static   3       3       3                                                               34h   True
   system   Static   2       2       2                                                               34h   True
   worker   Static   3       3       3                                                               34h   True
   ```
   {: .nowrap-default }

   Количество узлов `UPTODATE` должно быть равно количеству `NODES` и `READY`.

3. Образ контейнера в Deployment `deckhouse` имеет актуальный тег.

   Для проверки выполните команду:

   ```bash
   d8 k -n d8-system get deploy/deckhouse -oyaml | grep image
   ```
   Пример вывода:

   ```yml
   image: <your-cse-registry>/deckhouse/cse:v1.73.1
   ```

4. В веб-интерфейсе кластера отображается редакция платформы, новая версия платформы и релизный канал.

   <!-- Пример корректного отображения:  -->
   <!-- TODO: добавить скриншот -->

## Возможные проблемы и пути их решения

### Обновление cilium

В рамках обновления версии платформы v1.67.4 → v1.73.1 происходит обновление Cilium 1.14 → 1.17.
В случае если обновление застряло (поды `safe-agent-updater` в пространстве имён `d8-cni-cilium` находятся в состоянии `Init:4/5` на одном узле дольше 20 минут):

Выясните, для каких узлов DKP CSE запрашивает подтверждение на применение деструктивных изменений:

```bash
d8 k get nodes -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
```

Пример вывода:

```bash
example-cse-master-0.ru-central1.internal
example-cse-system-0.ru-central1.internal
```

Затем, удалите под `safe-agent-updater` на этих узлах.
 
Если в группе узлов установлен параметр `spec.disruptions.approvalMode=Auto`, то начнется поочередное обновление версии cilium на узлах с выполнением операций Cordon и Drain. Если в группе узлов установлен параметр `spec.disruptions.approvalMode=Manual`, то потребуется установить аннотацию `d8 k annotate node <NODE> update.node.deckhouse.io/disruption-approved=` на узел.

### Потеря одного etcd member во время миграции версии etcd с 3.5.17 на 3.6.1

В рамках обновления DKP CSE происходит апгрейд etcd с версии 3.5.17 на 3.6.1, что в некоторых случаях приводит к `CrashLoopBackOff` у одного из подов `d8-control-plane-manager`. Это происходит из-за того, что иногда member etcd после обновления остаётся в статусе unstarted и, как следствие, удаляется из кворума.

Если в процессе обновления один из подов `etcd-<nodename>` в пространстве имён `kube-system` переходит в состояние `CrashLoopBackOff` и в кворуме etcd уже нет участника с именем этого control-plane-узла, то нужно добавить узел etcd заново.

Просмотр участников кворума etcd:

```bash
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

Пример вывода с пропавшим участником кворума в кластере с 3 control-plane-узлами:

```bash
+------------------+---------+-------------------------------------------+---------------------------+--------------------------+-------------+
|        ID        | STATUS  |                    NAME                   |        PEER ADDRS         |       CLIENT ADDRS        | IS LEARNER |
+------------------+---------+-------------------------------------------+---------------------------+---------------------------+------------+
| 898edf6255f45679 | started | example-cse-master-1.ru-central1.internal | https://10.241.32.23:2380 | https://10.241.32.23:2379 |      false |
| 988286c43353ca92 | started | example-cse-master-0.ru-central1.internal | https://10.241.32.6:2380  | https://10.241.32.6:2379  |      false |
+------------------+---------+-------------------------------------------+---------------------------+---------------------------+------------+
```
{: .nowrap-default }

Для решения этой проблемы выполните следующие действия на проблемном control-plane-узле:

1. Перенесите папку с данными etcd `/var/lib/etcd`:

   ```bash
   mv /var/lib/etcd /tmp
   ```

2. Перенесите манифест статического пода etcd `/etc/kubernetes/manifests/etcd.yaml`:

   ```bash
   mv /etc/kubernetes/manifests/etcd.yaml /tmp
   ```

3. Перезапустите под `d8-control-plane-manager` на проблемном control-plane-узле.

   1. Получите имя пода: 

      ```bash
      d8 k -n kube-system get po -owide | grep control-plane | grep <node-name>
      ```

   2. Удалите под, имя которого было получено в предыдущем шаге:
  
      ```bash
      d8 k -n kube-system delete po <pod-name>
      ```

4. Убедитесь, что кворум etcd восстановлен:

   ```bash
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

   Целевое состояние — количество участников etcd равно количеству control-plane-узлов.

   Пример вывода для 3 control-plane-узлов:

   ```console
   +------------------+---------+-------------------------------------------+---------------------------+--------------------------+-------------+
   |        ID        | STATUS  |                    NAME                   |        PEER ADDRS         |       CLIENT ADDRS        | IS LEARNER |
   +------------------+---------+-------------------------------------------+---------------------------+---------------------------+------------+
   | 898edf6255f45679 | started | example-cse-master-1.ru-central1.internal | https://10.241.32.23:2380 | https://10.241.32.23:2379 |      false |
   | 988286c43353ca92 | started | example-cse-master-0.ru-central1.internal | https://10.241.32.6:2380  | https://10.241.32.6:2379  |      false |
   | bb3fcfe1f7464434 | started | example-cse-master-2.ru-central1.internal | https://10.241.32.21:2380 | https://10.241.32.21:2379 |      false |
   +------------------+---------+-------------------------------------------+---------------------------+---------------------------+------------+
   ```
   {: .nowrap-default }
{% endraw %}
