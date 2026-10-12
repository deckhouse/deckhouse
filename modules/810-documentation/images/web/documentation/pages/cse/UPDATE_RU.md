---
title: "Руководство по обновлению Deckhouse Platform Certified Security Edition"
permalink: ru/update.html
search: обновление, обновление платформы, обновление версии, upgrade, апгрейд, переход на новую версию, миграция, требования к узлам, системные требования, поддерживаемые ОС, РЕД ОС, Astra Linux, Альт, Московская серверная операционная система, действия перед обновлением, подготовка к обновлению, контроль целостности, включение контроля целостности, signature, Migrate, Enforce, containerd, ContainerdV2, cgroups, cgroups v2, erofs, cilium, etcd, etcd member, проверка успешности обновления, проблемы при обновлении, ЗПС, замкнутая программная среда, astra-digsig-control
searchBoost: 2
description: Руководство по обновлению Deckhouse Platform Certified Security Edition (DP CSE), подготовка к обновлению, проверка узлов кластера и действия после обновления.
lang: ru
---

<style>
.docs ol > li::marker {
  content: counters(list-item, ".") ". ";
}
</style>

Руководство описывает обновление Deckhouse Platform Certified Security Edition (DP CSE) [с версии v1.73.6 до v1.77.0](#обновление-dp-cse-с-версии-1736-на-версию-1770).

Если в кластере установлена более ранняя версия, сначала обновите DP CSE до v1.73.6:

- [с версии v1.67.4 до v1.73.6](#обновление-dp-cse-с-версии-1674-на-версию-1736);
- [с версии v1.73.x до v1.73.6](#обновление-dp-cse-с-версии-173x-на-версию-1736).
{% raw %}

## Минимальные требования к узлам кластера для обновления

Требования к аппаратной части:

- архитектура процессора x86-64;
- не менее 4 ядер CPU;
- не менее 12 ГБ RAM;
- не менее 50 ГБ дискового пространства.

Требования к программной части:

- РЕД ОС (не ниже 7.3);
- Astra Linux Special Edition (не ниже 1.7);
- Альт (не ниже 8 СП, релиз не ниже 10);
- Московская серверная операционная система (не ниже 15.5).

В DP CSE v1.77.0 все узлы используют среду выполнения контейнеров containerd v2. Перед обновлением до v1.77.0 убедитесь, что каждый узел кластера соответствует требованиям containerd v2:

- поддержка cgroups v2;
- ядро Linux версии 5.8 и новее, кроме версий 6.12.0–6.12.28 и 6.14.0–6.14.6 (уязвимость CVE-2025-37999 в EROFS);
- systemd версии 244 и новее;
- поддержка модуля ядра `erofs`;
- отсутствие пользовательских конфигураций в `/etc/containerd/conf.d`.

Как проверить узел, описано в разделе [«Обновление версии containerd»](#обновление-версии-containerd).

## С какой версии можно обновить DP CSE

DP CSE можно обновить до v1.77.0 только с последней доступной патч-версии DP CSE v1.73.x.

## Обновление DP CSE с версии 1.73.6 на версию 1.77.0

### Изменения, которые нужно учесть перед обновлением

- Минимальная поддерживаемая версия Kubernetes — 1.31. В параметре `kubernetesVersion` ClusterConfiguration допустимы значения `"1.31"`, `"1.34"` и `Automatic`. Значение `Automatic` соответствует версии 1.34.
- Среда выполнения контейнеров containerd v1 больше не поддерживается. DP CSE не установит релиз v1.77.0, пока containerd v1 используется на узлах, в группах узлов (`spec.cri.type: Containerd`) или в параметре `defaultCRI` ClusterConfiguration.
- Модуль `gost-integrity-controller` удалён. DP CSE не установит релиз v1.77.0, пока этот модуль включён.
- Модули `loki`, `log-shipper`, `prometheus`, `ingress-nginx`, `runtime-audit-engine`, `operator-trivy`, `virtualization`, `console` и другие поставляются как внешние модули через ModuleSource `deckhouse`. DP CSE не установит релиз v1.77.0, пока включённые модули из этого списка недоступны в ModuleSource `deckhouse`. Образы этих модулей загружаются в хранилище образов контейнеров вместе с образами DP CSE.
- Параметры `denyVulnerableImages` перенесены из модуля `admission-policy-engine` в модуль `operator-trivy`. В модуле `operator-trivy` запрет на запуск контейнеров из уязвимых образов по умолчанию выключен. Если секция `denyVulnerableImages` осталась в ModuleConfig `admission-policy-engine`, срабатывает алерт `VulnerableImagesDenialConfigNotMigrated`.
- Доступ к хранилищу образов контейнеров настраивается параметром `settings.registry` ModuleConfig `deckhouse`. Секция `deckhouse` в InitConfiguration устарела.
- Публикация API Kubernetes настраивается параметром `settings.apiserver.publishAPI` ModuleConfig `control-plane-manager`. Параметр `publishAPI` модуля `user-authn` устарел.
- Параметр `earlyOomEnabled` модуля `node-manager` удалён.
- Параметр `apiserver.signature` модуля `control-plane-manager` по умолчанию имеет значение `Migrate`. Значение, заданное в ModuleConfig явно, сохраняется.
- Политики модуля `admission-policy-engine` проверяют контейнеры, которые добавляются через субресурс `pods/ephemeralcontainers` (команда `d8 k debug`). В неймспейсах со строгими политиками отладочные контейнеры с правами root не запустятся.
- При включённом параметре `enableMultiTenancy` модуля `user-authz` итоговые права пользователя — объединение прав из ClusterAuthorizationRule (в разрешённых неймспейсах), AuthorizationRule, RoleBinding и ClusterRoleBinding.
- Нельзя создать или переименовать Group или User так, чтобы `Group.spec.name` или `User.spec.email` совпал с субъектом существующего AuthorizationRule или ClusterAuthorizationRule.
- Создавать, изменять и удалять SecurityPolicyException может только пользователь с уровнем доступа `ClusterAdmin`.
- Если API Kubernetes опубликован с включённой basic-аутентификацией (`enableBasicAuth`) в DexProvider, DP CSE отбрасывает группы, имена которых начинаются с `system:`.
- Аннотация `allow-access-to-kubernetes` у DexClient и DexAuthenticator учитывается, только если её значение — логическое `true`. DP CSE заново генерирует клиентский секрет DexAuthenticator с этой аннотацией. Поэтому пользователям связанных приложений может потребоваться войти повторно.
- Во время обновления перезапускаются компоненты control plane и модулей `cni-cilium`, `ingress-nginx`, `node-local-dns`, `kube-dns`, `metallb`, `vertical-pod-autoscaler` и других.

### Действия перед обновлением

1. Если в кластере есть узлы с Astra Linux, выполните действия из раздела [«Особенности обновления в Astra Linux»](#особенности-обновления-в-astra-linux).

1. Убедитесь, что в кластере установлена последняя патч-версия DP CSE v1.73.x (v1.73.6 на момент написания инструкции):

   ```bash
   d8 k get deckhouserelease
   ```

   Объект DeckhouseRelease `v1.73.6` должен находиться в фазе `Deployed`. Если установлена более ранняя версия, сначала [обновите DP CSE до последней патч-версии](#обновление-dp-cse-с-версии-173x-на-версию-1736).

1. Убедитесь, что в кластере нет алертов, кроме `SecurityEventsDetected`.

1. Убедитесь, что в очередях DP CSE нет ошибок:

   ```bash
   d8 system queue list
   ```

   Пример вывода, когда очереди пусты и ошибок нет:

   ```console
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Убедитесь, что на всех узлах используется containerd v2.

   1. Проверьте версию среды выполнения контейнеров на узлах:

      ```bash
      d8 k get nodes -o custom-columns=NAME:.metadata.name,CRI:.status.nodeInfo.containerRuntimeVersion
      ```

      Для всех узлов в столбце `CRI` должна быть указана версия `containerd://2.x`.

   1. Проверьте параметр `defaultCRI` в ClusterConfiguration:

      ```bash
      d8 k -n kube-system get secret d8-cluster-configuration -o jsonpath='{.data.cluster-configuration\.yaml}' | base64 -d | grep defaultCRI
      ```

      Параметр должен иметь значение `ContainerdV2` или `NotManaged`.

   1. Проверьте параметр `spec.cri.type` групп узлов:

      ```bash
      d8 k get ng -o custom-columns=NAME:.metadata.name,CRI:.spec.cri.type
      ```

      Ни у одной группы узлов не должно быть значения `Containerd`.

   Если хотя бы одна проверка не пройдена, переведите узлы на containerd v2 по инструкции из раздела [«Обновление версии containerd»](#обновление-версии-containerd). Дождитесь, пока обновятся все узлы.

1. Убедитесь, что модуль `gost-integrity-controller` выключен:

   ```bash
   d8 k get module gost-integrity-controller
   ```

   Если в столбце `ENABLED` указано `True`, выключите модуль:

   ```bash
   d8 system module disable gost-integrity-controller
   ```

1. Откройте ClusterConfiguration и проверьте параметр `kubernetesVersion`:

   ```bash
   d8 system edit cluster-configuration
   ```

   - Если параметр `kubernetesVersion` имеет значение `"1.29"` или `"1.30"`, установите значение `"1.31"`. Дождитесь, пока Kubernetes обновится на всех узлах и очереди DP CSE опустеют.
   - Если параметр `kubernetesVersion` имеет значение `"1.31"` или `Automatic`, ничего не меняйте.

   Проверьте версию Kubernetes на узлах:

   ```bash
   d8 k get no -o custom-columns=NAME:.metadata.name,VERSION:.status.nodeInfo.kubeletVersion
   ```

   На всех узлах должна быть версия не ниже `v1.31`.

1. Проверьте канал и режим обновлений в ModuleConfig `deckhouse`:

   ```bash
   d8 k edit mc deckhouse
   ```

   Убедитесь, что задан канал обновлений `LTS` (`spec.settings.releaseChannel: LTS`). Обновление возможно только на этом канале.

   В параметре `spec.settings.update.mode` задайте режим обновлений:

   - `AutoPatch` — автоматическое обновление патч-версий. Обновление минорной версии (например, с v1.73.6 на v1.77.0) требует подтверждения. Значение по умолчанию.
   - `Auto` — автоматическое обновление патч-версий и минорных версий с учётом окон обновлений (`spec.settings.update.windows`).
   - `Manual` — обновление любой версии требует подтверждения.

   Пример ModuleConfig `deckhouse` с каналом обновлений `LTS` и режимом `AutoPatch`:

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
       update:
         mode: AutoPatch
     version: 1
   ```

1. Загрузите обновление в хранилище образов контейнеров DP CSE.

   Скопируйте файлы поставки на компьютер, с которого есть доступ к хранилищу образов контейнеров в локальной сети. Файлы поставки можно взять из публичного хранилища образов контейнеров DP CSE, с USB-флеш-накопителя или с DVD-диска.

   Чтобы скопировать образы из публичного хранилища образов контейнеров DP CSE, выполните команду:

   ```bash
   d8 mirror pull <ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА> --source="registry-cse.deckhouse.ru/deckhouse/cse" --license="<ЛИЦЕНЗИОННЫЙ_КЛЮЧ>" --gost-digest --deckhouse-tag="v1.77.0"
   ```

   Не добавляйте в команду флаг `--include-module`. С этим флагом команда загружает только перечисленные в нём модули. Модули, которые нужны для работы кластера, не попадут в поставку.

   Здесь:

   - `<ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА>` — директория для скачанных образов;
   - `<ЛИЦЕНЗИОННЫЙ_КЛЮЧ>` — лицензионный ключ для доступа к публичному хранилищу образов контейнеров DP CSE.

   Команда загружает образы DP CSE и его модулей.

   Получите контрольные суммы файлов архива образов командой `d8 tools gostsum`. Сравните их с содержимым файлов с суффиксом `.gostsum`. Значения должны совпадать.

   Загрузите образы в хранилище образов контейнеров:

   ```bash
   d8 mirror push <ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА> <АДРЕС_ХРАНИЛИЩА>/<ПУТЬ_В_ХРАНИЛИЩЕ> --registry-login=<ИМЯ_ПОЛЬЗОВАТЕЛЯ> --registry-password=<ПАРОЛЬ>
   ```

   Здесь:

   - `<ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА>` — директория поставки с архивами образов DP CSE;
   - `<АДРЕС_ХРАНИЛИЩА>` — адрес хранилища образов контейнеров в локальной сети;
   - `<ПУТЬ_В_ХРАНИЛИЩЕ>` — путь к образам DP CSE в хранилище образов контейнеров, например `/deckhouse/cse`;
   - `<ИМЯ_ПОЛЬЗОВАТЕЛЯ>` — имя пользователя для авторизации в хранилище образов контейнеров;
   - `<ПАРОЛЬ>` — пароль пользователя для авторизации в хранилище образов контейнеров.

1. Убедитесь, что модули DP CSE доступны в ModuleSource `deckhouse`:

   ```bash
   d8 k get ms deckhouse -o jsonpath='{range .status.availableModules[*]}{.name}{"\t"}{.error}{"\n"}{end}'
   ```

   В списке должны быть все модули, включённые в кластере. Второй столбец (ошибки) должен быть пустым. ModuleSource синхронизируется с хранилищем образов контейнеров не сразу: это может занять несколько минут.

### Обновление версии DP CSE

1. Убедитесь, что в кластере появился объект DeckhouseRelease `v1.77.0`:

   ```bash
   d8 k get deckhouserelease
   ```

   Пример вывода с релизом `v1.77.0`, ожидающим установки:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME      PHASE        TRANSITIONTIME   MESSAGE
   v1.73.6   Deployed     12d
   v1.77.0   Pending      2m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Если в столбце `MESSAGE` указано невыполненное требование релиза, выполните это требование. Например, переведите узлы на containerd v2, выключите модуль `gost-integrity-controller` или дождитесь появления модулей в ModuleSource `deckhouse`.

1. Если задан режим обновлений `AutoPatch` или `Manual`, подтвердите обновление:

   ```bash
   d8 k patch deckhouserelease v1.77.0 --type=merge -p '{"approved":true}'
   ```

   Если заданы окна обновлений, обновление начнётся в ближайшее окно.

1. Дождитесь, пока очереди DP CSE опустеют и все узлы кластера обновятся:

   ```bash
   d8 system queue list
   d8 k get ng
   ```

   Для каждой группы узлов значения в столбцах `UPTODATE`, `NODES` и `READY` должны совпадать.

1. Если появится алерт `NodeRequiresDisruptionApprovalForUpdate`, выполните инструкцию из алерта.

### Действия после обновления

1. Включите запрет на запуск контейнеров из уязвимых образов в модуле `operator-trivy`.

   1. Получите ModuleConfig `admission-policy-engine`:

      ```bash
      d8 k get mc admission-policy-engine -o yaml
      ```

      Если в выводе есть секция `spec.settings.denyVulnerableImages`, запишите её значения. Затем удалите эту секцию:

      ```bash
      d8 k patch mc admission-policy-engine --type json -p='[{"op": "remove", "path": "/spec/settings/denyVulnerableImages"}]'
      ```

   1. Добавьте секцию `denyVulnerableImages` в ModuleConfig `operator-trivy`:

      ```bash
      d8 k edit mc operator-trivy
      ```

      Пример ModuleConfig `operator-trivy` с включённым запретом:

      ```yaml
      apiVersion: deckhouse.io/v1alpha1
      kind: ModuleConfig
      metadata:
        name: operator-trivy
      spec:
        enabled: true
        settings:
          denyVulnerableImages:
            enabled: true
        version: 2
      ```

      Если в удалённой секции были заданы параметры `allowedSeverityLevels` или `registrySecrets`, добавьте их с теми же значениями в секцию `denyVulnerableImages` ModuleConfig `operator-trivy`.

1. Переведите управление версией Kubernetes в автоматический режим:

   ```bash
   d8 system edit cluster-configuration
   ```

   Установите для параметра `kubernetesVersion` значение `Automatic`. Дождитесь, пока Kubernetes на всех узлах обновится до версии 1.34.

   - Узлы из групп, у которых параметр `spec.disruptions.approvalMode` равен `Auto`, обновятся автоматически.
   - Для узлов из групп, у которых параметр `spec.disruptions.approvalMode` равен `Manual`, появится алерт `NodeRequiresDisruptionApprovalForUpdate`. Чтобы обновить такие узлы, выполните инструкцию из алерта.

   Проверьте версию Kubernetes на узлах:

   ```bash
   d8 k get no -o custom-columns=NAME:.metadata.name,VERSION:.status.nodeInfo.kubeletVersion
   ```

   На всех узлах должна быть версия `v1.34`.

1. Если API Kubernetes опубликован с помощью параметра `publishAPI` модуля `user-authn`, перенесите настройки в параметр `settings.apiserver.publishAPI` ModuleConfig `control-plane-manager`.

1. [Проверьте, что обновление прошло успешно.](#как-понять-что-обновление-прошло-успешно)

## Обновление DP CSE с версии 1.67.4 на версию 1.73.6

### Действия перед обновлением

1. Если в кластере есть узлы с Astra Linux, выполните действия из раздела [«Особенности обновления в Astra Linux»](#особенности-обновления-в-astra-linux).

1. Загрузите обновление в хранилище образов контейнеров DP CSE.

   Скопируйте файлы поставки на компьютер, с которого есть доступ к хранилищу образов контейнеров в локальной сети. Файлы поставки можно взять из публичного хранилища образов контейнеров DP CSE, с USB-флеш-накопителя или с DVD-диска.

   Чтобы скопировать образы из публичного хранилища образов контейнеров DP CSE, выполните команду:

   ```bash
   d8 mirror pull <ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА> --source="registry-cse.deckhouse.ru/deckhouse/cse" --license="<ЛИЦЕНЗИОННЫЙ_КЛЮЧ>" --gost-digest --deckhouse-tag="v1.73.6" --include-module=stronghold
   ```

   Здесь:

   - `<ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА>` — директория для скачанных образов;
   - `<ЛИЦЕНЗИОННЫЙ_КЛЮЧ>` — лицензионный ключ для доступа к публичному хранилищу образов контейнеров DP CSE.

   Получите контрольные суммы файлов архива образов командой `d8 tools gostsum`. Сравните их с содержимым файлов с суффиксом `.gostsum`. Значения должны совпадать.

   Загрузите образы в хранилище образов контейнеров:

   ```bash
   d8 mirror push <ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА> <АДРЕС_ХРАНИЛИЩА>/<ПУТЬ_В_ХРАНИЛИЩЕ> --registry-login=<ИМЯ_ПОЛЬЗОВАТЕЛЯ> --registry-password=<ПАРОЛЬ>
   ```

   Здесь:

   - `<ПУТЬ_К_ДИРЕКТОРИИ_ПАКЕТА>` — директория поставки с архивами образов DP CSE;
   - `<АДРЕС_ХРАНИЛИЩА>` — адрес хранилища образов контейнеров в локальной сети;
   - `<ПУТЬ_В_ХРАНИЛИЩЕ>` — путь к образам DP CSE в хранилище образов контейнеров, например `/deckhouse/cse`;
   - `<ИМЯ_ПОЛЬЗОВАТЕЛЯ>` — имя пользователя для авторизации в хранилище образов контейнеров;
   - `<ПАРОЛЬ>` — пароль пользователя для авторизации в хранилище образов контейнеров.

1. Убедитесь, что в кластере нет алертов, кроме `D8DeckhouseIsNotOnReleaseChannel`.

1. Убедитесь, что в очередях DP CSE нет ошибок:

   ```bash
   d8 system queue list
   ```

   Пример вывода, когда очереди пусты и ошибок нет:

   ```console
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Если в кластере есть NodeGroupConfiguration `sysctl-tune-fstec`, добавьте к нему аннотации и лейбл.

   Проверьте, есть ли в кластере NodeGroupConfiguration `sysctl-tune-fstec`:

   ```bash
   d8 k get ngc | grep sysctl-tune-fstec
   ```

   Пример вывода, если NodeGroupConfiguration есть в кластере:

   ```console
   sysctl-tune-fstec          100      ["*"]        ["*"]
   ```

   Если вывод пустой, пропустите этот шаг. Если NodeGroupConfiguration есть в кластере, добавьте аннотации и лейбл:

   ```bash
   d8 k annotate ngc sysctl-tune-fstec meta.helm.sh/release-namespace=d8-system
   d8 k annotate ngc sysctl-tune-fstec meta.helm.sh/release-name=node-manager
   d8 k label ngc sysctl-tune-fstec app.kubernetes.io/managed-by=Helm
   ```

1. Если модуль `loki` включён, увеличьте размер его хранилища данных до 50 ГБ. Это нужно, только если хранилище данных модуля `loki` использует драйвер `local-path-provisioner`, `csi-nfs`, `sds-local-volume` или `csi-ceph`.

   1. Проверьте, включён ли модуль `loki`:

      ```bash
      d8 k get module | grep loki
      ```

      Пример вывода для выключенного модуля:

      ```console
      loki           462      Embedded   Downloaded   False     False
      ```

      Пример вывода для включённого модуля:

      ```console
      loki           462      Embedded   Ready        True      True
      ```

      Если модуль `loki` выключен, пропустите остальные шаги этого пункта.

   1. Определите драйвер хранилища данных модуля `loki`. Для этого получите провизионер StorageClass, который использует PVC модуля `loki`:

      ```bash
      d8 k get sc $(d8 k -n d8-monitoring get pvc storage-loki-0 -o jsonpath='{.spec.storageClassName}') -o jsonpath='{.provisioner}{"\n"}'
      ```

      Пример вывода:

      ```console
      deckhouse.io/localpath
      ```

      Найдите драйвер по провизионеру:

      | Провизионер | Драйвер хранилища |
      | --- | --- |
      | `deckhouse.io/<ИМЯ_LOCALPATHPROVISIONER>` | `local-path-provisioner` |
      | `nfs.csi.k8s.io` | `csi-nfs` |
      | `local.csi.storage.deckhouse.io` | `sds-local-volume` |
      | `rbd.csi.ceph.com` или `cephfs.csi.ceph.com` | `csi-ceph` |

      Здесь `<ИМЯ_LOCALPATHPROVISIONER>` — имя ресурса LocalPathProvisioner, например `localpath`.

      Если провизионера нет в таблице, пропустите остальные шаги этого пункта.

   1. Убедитесь, что в хранилище есть не менее 50 ГБ свободного места для расширения хранилища данных модуля `loki`.

   1. Измените размер PVC модуля `loki`:

      ```bash
      d8 k -n d8-monitoring edit pvc storage-loki-0
      ```

      Установите параметр `spec.resources.requests.storage: 50Gi`, как в примере:

      ```yaml
      # ...
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 50Gi
      # ...
      ```

   1. Этот шаг выполняйте **только** для драйвера `local-path-provisioner`. Для `csi-nfs`, `sds-local-volume` и `csi-ceph` **пропустите его:** драйвер сам изменит размер PV после изменения PVC.

      Измените размер PV, который использует модуль `loki`:

      1. Получите имя PV:

         ```bash
         d8 k get pv | grep loki
         ```

         Пример вывода:

         ```console
         pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd   2Gi        RWO            Delete           Bound    d8-monitoring/storage-loki-0          localpath           <unset>                          3m40s
         ```

         Выберите строку с `d8-monitoring/storage-loki-0`. Имя PV указано в первом столбце. В этом примере имя PV — `pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd`.

      1. Измените размер PV:

         ```bash
         d8 k edit pv <ИМЯ_PV>
         ```

         Здесь `<ИМЯ_PV>` — имя PV, полученное на предыдущем шаге.

         Установите параметр `spec.capacity.storage: 50Gi`, как в примере:

         ```yaml
         # ...
         spec:
           accessModes:
           - ReadWriteOnce
           capacity:
             storage: 50Gi
           claimRef:
         # ...
         ```

   1. Убедитесь, что размер PV модуля `loki` изменился:

      ```bash
      d8 k get pv | grep loki
      ```

      Во втором столбце (`CAPACITY`) должно быть указано `50Gi`. Пример вывода:

      <!-- markdownlint-disable MD031 -->
      ```console
      pvc-09ea47fe-b87a-4bc7-985b-fe2c319c11cd   50Gi       RWO            Delete           Bound    d8-monitoring/storage-loki-0          localpath           <unset>                          11m
      ```
      {: .nowrap-default }
      <!-- markdownlint-enable MD031 -->

   1. Удалите StatefulSet `loki`:

      ```bash
      d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-monitoring delete sts loki
      ```

      Убедитесь, что поды `loki` удалены:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Вывод команды должен быть пустым.

   1. Добавьте в ModuleConfig `loki` параметр `spec.settings.diskSizeGigabytes: 50`:

      ```bash
      d8 k edit mc loki
      ```

      Пример ModuleConfig `loki` с новым размером диска:

      ```yaml
      # ...
      spec:
        enabled: true
        settings:
          diskSizeGigabytes: 50
          # ...
        version: 1
      # ...
      ```

   1. Дождитесь, пока под `loki` перейдёт в состояние `Running`:

      ```bash
      d8 k -n d8-monitoring get po | grep loki
      ```

      Пример вывода с запущенным подом:

      ```console
      loki-0                   2/2     Running   0             5m28s
      ```

1. Переведите управление версией Kubernetes в ручной режим.

   1. Проверьте версию Kubernetes в ClusterConfiguration:

      ```bash
      d8 system edit cluster-configuration
      ```

      - Если параметр `kubernetesVersion` имеет значение `"1.29"`, ничего не меняйте.
      - Если параметр `kubernetesVersion` имеет значение `"1.27"`, установите значение `"1.29"`. Дождитесь, пока Kubernetes обновится на всех узлах и очереди DP CSE опустеют.
      - Если параметр `kubernetesVersion` имеет значение `Automatic`, установите значение `"1.29"`.

   1. Убедитесь, что очереди DP CSE пусты:

      ```bash
      d8 system queue list
      ```

      Пример вывода, когда очереди пусты и ошибок нет:

      ```console
      Summary:
      - 'main' queue: empty.
      - 103 other queues (0 active, 103 empty): 0 tasks.
      - no tasks to handle.
      ```

   1. Убедитесь, что все узлы кластера обновлены:

      ```bash
      d8 k get ng
      ```

      Пример вывода команды с успешно обновлёнными узлами:

      <!-- markdownlint-disable MD031 -->
      ```console
      NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
      master   Static   3       3       3                                                               34h   True
      system   Static   2       2       2                                                               34h   True
      worker   Static   3       3       3                                                               34h   True
      ```
      {: .nowrap-default }
      <!-- markdownlint-enable MD031 -->

      Для каждой группы узлов значения в столбцах `UPTODATE`, `NODES` и `READY` должны совпадать.

1. Убедитесь, что в ModuleConfig `deckhouse` не задан канал обновлений.

   Получите ModuleConfig `deckhouse`:

   ```bash
   d8 k get mc deckhouse -o yaml
   ```

   Пример вывода, когда канал обновлений не задан:

   ```console
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     creationTimestamp: "<ВРЕМЯ_СОЗДАНИЯ>"
     finalizers:
     - modules.deckhouse.io/module-config
     generation: 1
     name: deckhouse
     resourceVersion: "<ВЕРСИЯ_ОБЪЕКТА>"
     uid: <UID_ОБЪЕКТА>
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

   Здесь `<ВРЕМЯ_СОЗДАНИЯ>`, `<ВЕРСИЯ_ОБЪЕКТА>` и `<UID_ОБЪЕКТА>` — значения, которые зависят от кластера.

   Если задан параметр `spec.settings.releaseChannel`, удалите его:

   ```bash
   d8 k patch mc deckhouse --type='merge' -p='{"spec":{"settings":{"releaseChannel": null}}}'
   ```

   Пример вывода команды:

   ```console
   moduleconfig.deckhouse.io/deckhouse patched
   ```

### Обновление версии DP CSE

1. Примените NodeGroupConfiguration `bashible-migrations-002.sh`. Этот NodeGroupConfiguration перезапускает службу `bashible` на узлах, где нет функции `bb-d8-node-name`:

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

1. Замените образ в Deployment `deckhouse`:

   ```bash
   d8 k --as=system:serviceaccount:d8-system:deckhouse set image deployment -n d8-system deckhouse deckhouse=<АДРЕС_ХРАНИЛИЩА>/<ПУТЬ_В_ХРАНИЛИЩЕ>:v1.73.6
   ```

   Здесь:

   - `<АДРЕС_ХРАНИЛИЩА>` — адрес хранилища образов контейнеров в локальной сети;
   - `<ПУТЬ_В_ХРАНИЛИЩЕ>` — путь к образам DP CSE в хранилище образов контейнеров, например `/deckhouse/cse`.

1. Дождитесь, пока очереди DP CSE опустеют и все узлы кластера обновятся.

   1. Убедитесь, что очереди DP CSE пусты:

      ```bash
      d8 system queue list
      ```

      Пример вывода, когда очереди пусты и ошибок нет:

      ```console
      Summary:
      - 'main' queue: empty.
      - 103 other queues (0 active, 103 empty): 0 tasks.
      - no tasks to handle.
      ```

   1. Убедитесь, что все узлы кластера обновлены:

      ```bash
      d8 k get ng
      ```

      Пример вывода команды с успешно обновлёнными узлами:

      <!-- markdownlint-disable MD031 -->
      ```console
      NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
      master   Static   3       3       3                                                               34h   True
      system   Static   2       2       2                                                               34h   True
      worker   Static   3       3       3                                                               34h   True
      ```
      {: .nowrap-default }
      <!-- markdownlint-enable MD031 -->

      Для каждой группы узлов значения в столбцах `UPTODATE`, `NODES` и `READY` должны совпадать.

1. Если появится алерт `NodeRequiresDisruptionApprovalForUpdate`, выполните инструкцию из алерта.

1. В ModuleConfig `control-plane-manager` измените значение параметра `spec.version` с `1` на `2`:

   ```bash
   d8 k edit mc control-plane-manager
   ```

   Пример ModuleConfig `control-plane-manager` с версией `2`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     settings:
       # ...
     version: 2
   ```

1. Обновите версию Kubernetes. Откройте ClusterConfiguration на редактирование:

   ```bash
   d8 system edit cluster-configuration
   ```

   Установите для параметра `kubernetesVersion` значение `Automatic`. Дождитесь, пока Kubernetes обновится на всех узлах.

   - Узлы из групп, у которых параметр `spec.disruptions.approvalMode` равен `Auto`, обновятся автоматически.
   - Для узлов из групп, у которых параметр `spec.disruptions.approvalMode` равен `Manual`, появится алерт `NodeRequiresDisruptionApprovalForUpdate`. Чтобы обновить такие узлы, выполните инструкцию из алерта.

   Проверьте версию Kubernetes на узлах:

   ```bash
   d8 k get no -o custom-columns=NAME:.metadata.name,VERSION:.status.nodeInfo.kubeletVersion
   ```

   Пример вывода:

   ```console
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

   Убедитесь, что все узлы кластера обновлены:

   ```bash
   d8 k get ng
   ```

   Пример вывода команды с успешно обновлёнными узлами:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   master   Static   3       3       3                                                               34h   True
   system   Static   2       2       2                                                               34h   True
   worker   Static   3       3       3                                                               34h   True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Для каждой группы узлов значения в столбцах `UPTODATE`, `NODES` и `READY` должны совпадать.

1. Удалите NodeGroupConfiguration `bashible-migrations-002.sh`:

   ```bash
   d8 k delete ngc bashible-migrations-002.sh
   ```

1. Задайте канал обновлений в ModuleConfig `deckhouse`. Откройте ModuleConfig на редактирование:

   ```bash
   d8 k edit mc deckhouse
   ```

   Установите параметр `spec.settings.releaseChannel: LTS`.

   Пример ModuleConfig `deckhouse` с каналом обновлений `LTS`:

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

1. [Проверьте, что обновление прошло успешно.](#как-понять-что-обновление-прошло-успешно)

### Включение контроля целостности

Чтобы включить контроль целостности, выполните миграцию данных etcd:

1. Переведите контроль целостности в режим `Migrate`. Для этого в ModuleConfig `control-plane-manager` установите для параметра `settings.apiserver.signature` значение `Migrate`:

   ```bash
   d8 k edit mc control-plane-manager
   ```

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
   # ...
   ```

1. Дождитесь, пока очереди DP CSE опустеют:

   ```bash
   d8 system queue list
   ```

   Пример вывода, когда очереди пусты и ошибок нет:

   ```console
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Выполните миграцию данных.

   Миграцию выполняет команда `d8 tools sig-migrate`. Команда составляет список всех объектов Kubernetes и по очереди добавляет к ним аннотацию `d8-migration=<ВРЕМЕННАЯ_МЕТКА>`. Затем команда удаляет аннотации с префиксом `d8-migration-`. Здесь `<ВРЕМЕННАЯ_МЕТКА>` — временная метка, которую устанавливает команда.

   Время миграции зависит от размера кластера и нагрузки на него.

   {% endraw %}
   {% alert level="warning" %}
   Если пропадёт сетевое соединение или завершится SSH-сессия, миграция прервётся. Запускайте миграцию в терминальном мультиплексоре `screen` или `tmux`.
   {% endalert %}
   {% raw %}

   Если на master-узле нет утилит `screen` и `tmux`, установите одну из них.

   Запустите миграцию от имени пользователя root на master-узле кластера:

   ```bash
   d8 tools sig-migrate
   ```

   Если команде не удалось добавить аннотацию хотя бы к одному объекту, после миграции она выводит предупреждение. В предупреждении указаны:

   - количество объектов, которые не удалось мигрировать;
   - пути к файлам с логами ошибок;
   - как найти причину ошибок и повторно запустить миграцию.

   Пример вывода при успешном завершении миграции:

   ```console
   Retrying failed annotations from previous runs...
   Loaded 3 objects for retry from /tmp/failed_annotations.txt.
   Progress: [100%] Annotating: Kind=nodegroups, Namespace=clusterwide, Name=system
   ```

   Пример вывода с ошибками:

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

   Если миграция завершилась с ошибками, запустите её повторно с флагом `--retry`:

   ```bash
   d8 tools sig-migrate --retry
   ```

   Повторяйте команду `d8 tools sig-migrate --retry`, пока не выполнятся оба условия:

   - команда не сообщает об объектах с ошибками;
   - алерт `D8SignatureErrorsDetected` не срабатывает.

   Если после трёх повторных запусков ошибки остаются, замените в файле `/tmp/failed_annotations.txt` версию API объектов NodeGroup на `v1`:

   ```bash
   sed -i '/nodegroups/s/v1alpha[^|]*/v1/g' /tmp/failed_annotations.txt
   ```

   Затем повторите миграцию:

   ```bash
   d8 tools sig-migrate --retry
   ```

   Миграция изменяет каждый объект в кластере. Если из-за сбоя в кластере какой-либо объект нельзя изменить, миграция не завершится. В этом случае устраните причину, по которой объект нельзя изменить, и повторите миграцию.

   Если ошибки миграции повторяются, обратитесь в техническую поддержку.

1. Переведите контроль целостности в режим `Enforce`. Для этого в ModuleConfig `control-plane-manager` установите для параметра `settings.apiserver.signature` значение `Enforce`:

   ```bash
   d8 k edit mc control-plane-manager
   ```

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
   # ...
   ```

1. Дождитесь, пока очереди DP CSE опустеют:

   ```bash
   d8 system queue list
   ```

   Пример вывода, когда очереди пусты и ошибок нет:

   ```console
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

### Обновление версии containerd

В DP CSE v1.77.0 containerd v1 не поддерживается. Перед обновлением до v1.77.0 переведите все узлы кластера на containerd v2. В DP CSE v1.73.6 containerd v2 можно включить для всего кластера или для отдельных групп узлов.

Требования к узлам для перехода на containerd v2:

- поддержка cgroups v2;
- ядро Linux версии 5.8 и новее, кроме версий 6.12.0–6.12.28 и 6.14.0–6.14.6 (уязвимость CVE-2025-37999 в EROFS);
- systemd версии 244 и новее;
- поддержка модуля ядра `erofs`;
- отсутствие пользовательских конфигураций в `/etc/containerd/conf.d`.

DP CSE проверяет, есть ли в версии ядра Linux известные уязвимости. Если версия ядра уязвима, DP CSE не разрешает использовать containerd v2 на узле, даже если остальные требования выполнены. Уязвимость устранена в версиях ядра 6.12.29 и 6.14.7 и в более новых версиях этих веток. Уязвимые версии ядра могут быть, например, в РЕД ОС.

Если узел не соответствует хотя бы одному из требований, DP CSE добавляет на узел лейбл `node.deckhouse.io/containerd-v2-unsupported`. Если на узле есть пользовательские конфигурации в `/etc/containerd/conf.d`, DP CSE добавляет на узел лейбл `node.deckhouse.io/containerd-config=custom`.

Если на узле есть один из этих лейблов, изменить параметр `spec.cri.type` его группы узлов нельзя. Получите список узлов, которые не соответствуют требованиям:

```bash
d8 k get node -l node.deckhouse.io/containerd-v2-unsupported
d8 k get node -l node.deckhouse.io/containerd-config=custom
```

Чтобы проверить отдельный узел на соответствие требованиям, выполните на нём команды:

```bash
uname -r | cut -d- -f1
stat -f -c %T /sys/fs/cgroup
systemctl --version | awk 'NR==1{print $2}'
modprobe -qn erofs && echo "TRUE" || echo "FALSE"
ls -l /etc/containerd/conf.d
```

Узел соответствует требованиям, если команды вывели следующие результаты:

- `uname -r` — версия 5.8 или новее, кроме 6.12.0–6.12.28 и 6.14.0–6.14.6;
- `stat` — `cgroup2fs`;
- `systemctl --version` — 244 или больше;
- `modprobe` — `TRUE`;
- `ls -l` — директория пуста или не существует.

Включите containerd v2 одним из способов:

- Для всего кластера.

  Укажите значение `ContainerdV2` в параметре `defaultCRI` ClusterConfiguration. Это значение применяется ко всем NodeGroup, в которых явно не задан параметр `spec.cri.type`.

  Откройте ClusterConfiguration на редактирование:

  ```bash
  d8 system edit cluster-configuration
  ```

  Пример ClusterConfiguration с containerd v2 по умолчанию:

  ```yaml
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  # ...
  defaultCRI: ContainerdV2
  # ...
  ```

- Для конкретной группы узлов.

  Укажите значение `ContainerdV2` в параметре `spec.cri.type` NodeGroup.

  Например, чтобы включить containerd v2 для группы узлов `worker`, откройте NodeGroup `worker` на редактирование:

  ```bash
  d8 k edit ng worker
  ```

  Пример NodeGroup `worker` с containerd v2:

  ```yaml
  apiVersion: deckhouse.io/v1
  kind: NodeGroup
  metadata:
    name: worker
  spec:
    cri:
      type: ContainerdV2
  # ...
  ```

После перехода на containerd v2 DP CSE обновляет узлы по очереди. Обновление узла прерывает работу нагрузки на этом узле. Такое изменение называется деструктивным. Как обновится узел, зависит от параметра `spec.disruptions.approvalMode` его группы узлов (NodeGroup):

- Узлы из групп, у которых параметр `spec.disruptions.approvalMode` равен `Auto`, обновятся автоматически.
- Для узлов из групп, у которых параметр `spec.disruptions.approvalMode` равен `Manual`, появится алерт `NodeRequiresDisruptionApprovalForUpdate`. Чтобы обновить такие узлы, выполните инструкцию из алерта.

### Возможные проблемы и их решение

#### Обновление Cilium

При обновлении DP CSE с версии v1.67.4 на v1.73.x Cilium обновляется с версии 1.14 на 1.17.

Обновление Cilium зависло, если поды `safe-agent-updater` в неймспейсе `d8-cni-cilium` находятся в состоянии `Init:4/5` на одном узле дольше 20 минут. В этом случае получите список узлов, для которых DP CSE ждёт подтверждения деструктивных изменений:

```bash
d8 k get nodes -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
```

Пример вывода:

```console
example-cse-master-0.ru-central1.internal
example-cse-system-0.ru-central1.internal
```

Удалите поды `safe-agent-updater` на этих узлах.

Если у группы узлов параметр `spec.disruptions.approvalMode` равен `Auto`, DP CSE по очереди обновит Cilium на узлах. Во время обновления DP CSE выполняет cordon и drain каждого узла.

Если у группы узлов параметр `spec.disruptions.approvalMode` равен `Manual`, добавьте каждому узлу из списка аннотацию `update.node.deckhouse.io/disruption-approved=`:

```bash
d8 k annotate node <ИМЯ_УЗЛА> update.node.deckhouse.io/disruption-approved=
```

Здесь `<ИМЯ_УЗЛА>` — имя узла из списка.

#### Потеря одного участника etcd при обновлении etcd с версии 3.5.17 на 3.6.1

Во время обновления DP CSE etcd обновляется с версии 3.5.17 до 3.6.1. После обновления участник etcd иногда остаётся в статусе `unstarted` и поэтому удаляется из кворума. Тогда один из подов `d8-control-plane-manager` переходит в состояние `CrashLoopBackOff`.

Признак проблемы: под `etcd-<ИМЯ_УЗЛА>` в неймспейсе `kube-system` перешёл в состояние `CrashLoopBackOff`. Здесь `<ИМЯ_УЗЛА>` — имя master-узла.

Проверьте, есть ли этот master-узел в кворуме etcd. Получите список участников кворума etcd:

```bash
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

Пример вывода для кластера с тремя master-узлами, в котором одного участника нет в кворуме:

<!-- markdownlint-disable MD031 -->
```console
+------------------+---------+-------------------------------------------+---------------------------+--------------------------+-------------+
|        ID        | STATUS  |                    NAME                   |        PEER ADDRS         |       CLIENT ADDRS        | IS LEARNER |
+------------------+---------+-------------------------------------------+---------------------------+---------------------------+------------+
| 898edf6255f45679 | started | example-cse-master-1.ru-central1.internal | https://10.241.32.23:2380 | https://10.241.32.23:2379 |      false |
| 988286c43353ca92 | started | example-cse-master-0.ru-central1.internal | https://10.241.32.6:2380  | https://10.241.32.6:2379  |      false |
+------------------+---------+-------------------------------------------+---------------------------+---------------------------+------------+
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Если участника с именем этого master-узла нет в списке, восстановите участника etcd. Выполните на этом master-узле следующие действия:

1. Переместите директорию с данными etcd `/var/lib/etcd` в `/tmp`:

   ```bash
   mv /var/lib/etcd /tmp
   ```

1. Переместите манифест статического пода etcd `/etc/kubernetes/manifests/etcd.yaml` в `/tmp`:

   ```bash
   mv /etc/kubernetes/manifests/etcd.yaml /tmp
   ```

1. Перезапустите под `d8-control-plane-manager` на этом master-узле.

   1. Получите имя пода:

      ```bash
      d8 k -n kube-system get po -owide | grep control-plane | grep <ИМЯ_УЗЛА>
      ```

      Здесь `<ИМЯ_УЗЛА>` — имя master-узла, который нужно восстановить.

   1. Удалите под:

      ```bash
      d8 k -n kube-system delete po <ИМЯ_ПОДА>
      ```

      Здесь `<ИМЯ_ПОДА>` — имя пода `d8-control-plane-manager` из вывода предыдущей команды.

1. Убедитесь, что кворум etcd восстановлен:

   ```bash
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

   Количество участников etcd должно быть равно количеству master-узлов.

   Пример вывода для кластера с тремя master-узлами:

   <!-- markdownlint-disable MD031 -->
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
   <!-- markdownlint-enable MD031 -->

## Обновление DP CSE с версии 1.73.x на версию 1.73.6

### Действия перед обновлением

1. Убедитесь, что в кластере нет алертов, кроме `SecurityEventsDetected`.

1. Убедитесь, что в очередях DP CSE нет ошибок:

   ```bash
   d8 system queue list
   ```

   Пример вывода, когда очереди пусты и ошибок нет:

   ```console
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Задайте параметры обновления в секции `spec.settings.update` ModuleConfig `deckhouse`. Откройте ModuleConfig `deckhouse` на редактирование:

   ```bash
   d8 k edit mc deckhouse
   ```

   Пример секции `spec.settings.update` с окнами обновлений по вторникам и субботам с 8:00 до 15:00:

   ```yaml
   # ...
   spec:
     settings:
       update:
         windows:
         - from: '8:00'
           to: '15:00'
           days:
           - Tue
           - Sat
   # ...
   ```

   Режим обновлений DP CSE задаёт параметр `spec.settings.update.mode`. Режим работает, только если задан канал обновлений. Допустимые значения:

   - `AutoPatch` — автоматическое обновление патч-версий. Обновление минорной версии (например, с `v1.69.*` на `v1.70.x`) требует подтверждения. Если заданы окна обновлений, патч-версии (например, с v1.70.1 на v1.70.2) обновляются в эти окна. Значение по умолчанию.
   - `Auto` — автоматическое обновление всех версий: минорных (например, с `v1.69.*` на `v1.70.x`) и патч-версий (например, с v1.70.1 на v1.70.2). Если заданы окна обновлений, версии обновляются в эти окна. Если окна не заданы, версии обновляются сразу после появления на канале обновлений.
   - `Manual` — ручное обновление всех версий. Обновление каждой минорной и патч-версии требует подтверждения.

   Чтобы подтвердить обновление, установите параметр `approved: true` в DeckhouseRelease той версии, до которой обновляется DP CSE.

### Обновление версии DP CSE

1. Если в кластере есть узлы с Astra Linux, выполните действия из раздела [«Особенности обновления в Astra Linux»](#особенности-обновления-в-astra-linux).

1. Загрузите обновление в хранилище образов контейнеров DP CSE. Для этого выполните шаг «Загрузите обновление в хранилище образов контейнеров DP CSE» из подраздела «Действия перед обновлением» раздела [«Обновление DP CSE с версии 1.67.4 на версию 1.73.6»](#обновление-dp-cse-с-версии-1674-на-версию-1736):

   - скопируйте образы командой `d8 mirror pull`;
   - проверьте контрольные суммы командой `d8 tools gostsum`;
   - загрузите образы в хранилище образов контейнеров командой `d8 mirror push`.

1. Дождитесь обновления. DP CSE обновится с учётом заданных режима и окон обновлений.

1. [Проверьте, что обновление прошло успешно.](#как-понять-что-обновление-прошло-успешно)

## Как понять, что обновление прошло успешно

1. Убедитесь, что очереди DP CSE пусты:

   ```bash
   d8 system queue list
   ```

   Пример вывода, когда очереди пусты и ошибок нет:

   ```console
   Summary:
   - 'main' queue: empty.
   - 103 other queues (0 active, 103 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Убедитесь, что все узлы в кластере обновлены:

   ```bash
   d8 k get ng
   ```

   Пример вывода команды с успешно обновлёнными узлами:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   master   Static   3       3       3                                                               34h   True
   system   Static   2       2       2                                                               34h   True
   worker   Static   3       3       3                                                               34h   True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Для каждой группы узлов значения в столбцах `UPTODATE`, `NODES` и `READY` должны совпадать.

1. Убедитесь, что DeckhouseRelease целевой версии перешёл в фазу `Deployed`:

   ```bash
   d8 k get deckhouserelease
   ```

   Пример вывода после обновления до v1.77.0:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME      PHASE         TRANSITIONTIME   MESSAGE
   v1.73.6   Superseded    1d6h
   v1.77.0   Deployed      1h
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Убедитесь, что в веб-интерфейсе DP CSE отображаются редакция DP CSE, новая версия DP CSE и канал обновлений.

   <!-- Пример корректного отображения:  -->

## Особенности обновления в Astra Linux

Перед обновлением отключите режим замкнутой программной среды (ЗПС) на всех узлах с Astra Linux, где он включён.

Чтобы проверить, включён ли режим ЗПС, выполните на узле команды:

```bash
astra-digsig-control is-enabled
cat /sys/digsig/elf_mode
```

Отключите режим ЗПС одним из способов:

- Вручную. Выполните на узле команды:

  ```bash
  sed -i '/^DIGSIG_ELF_MODE=/d' /etc/digsig/digsig_initramfs.conf && echo 'DIGSIG_ELF_MODE=0' | tee -a /etc/digsig/digsig_initramfs.conf
  update-initramfs -u -k all
  systemctl reboot
  ```

- Утилитой `astra-digsig-control`. Выполните на узле команду:

  ```bash
  astra-digsig-control disable
  ```

**После обновления DP CSE** создайте NodeGroupConfiguration `astra-zps.sh`. Этот NodeGroupConfiguration включает режим ЗПС на узлах кластера с Astra Linux:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: astra-zps.sh
spec:
  weight: 98
  # Чтобы включить режим ЗПС только на части узлов, укажите имена NodeGroup.
  nodeGroups: [ "*" ]
  bundles: [ "astra" ]
  content: |
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

NodeGroupConfiguration `astra-zps.sh` выполняет на узлах кластера с Astra Linux следующие действия:

- проверяет, загружен ли модуль `digsig_verif` в ОС узла;
- добавляет публичные ключи для проверки подписи бинарных файлов компонентов DP CSE;
- включает проверку подписи бинарных файлов;
- обновляет образы initramfs для всех установленных в системе ядер;
- перезагружает узел.
{% endraw %}
