---
title: "Управление control plane"
permalink: ru/admin/configuration/platform-scaling/control-plane.html
lang: ru
---

## Основные возможности

Deckhouse Kubernetes Platform (DKP) осуществляет управление компонентами control plane кластера с помощью модуля `control-plane-manager`, который запускается на всех master-узлах (узлы с лейблом `node-role.kubernetes.io/control-plane: ""`).

Функционал управления control plane включает:

- Управление сертификатами необходимых для работы control plane, в том числе их продление и выпуск при изменении конфигурации. Автоматически поддерживается безопасная конфигурация и возможность быстрого добавления дополнительных SAN для организации защищённого доступа к API Kubernetes.

- Настройка компонентов. DKP генерирует все необходимые конфигурации и манифесты (kube-apiserver, etcd и др.), снижая вероятность ручных ошибок.

- Upgrade/downgrade компонентов. DKP поддерживает согласованное обновление или понижение версии control plane, что позволяет поддерживать единообразие версий в кластере.

- Управление конфигурацией etcd-кластера и его членов. DKP масштабирует master-узлы, выполняет миграцию из single-master в multi-master и обратно.

- Настройка kubeconfig — DKP формирует актуальный конфигурационный файл (с правами `cluster-admin`), автоматическое продление и обновление, а также создание `symlink` для пользователя `root`.

> Некоторые параметры, влияющие на работу Control Plane, берутся из ресурса ClusterConfiguration.

## Включение, отключение и настройка модуля

### Включение / отключение

Включить или выключить модуль `control-plane-manager` можно следующими способами:

1. Создайте (или измените) ресурс ModuleConfig/control-plane-manager, указав `spec.enabled: true` или `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     enabled: true
   ```

1. Используйте команду:

   ```bash
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- \
    deckhouse-controller module enable control-plane-manager
   ```

   или

   ```bash
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller module disable control-plane-manager
   ```

1. Через [веб-интерфейс Deckhouse](https://deckhouse.ru/products/kubernetes-platform/modules/console/stable/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `control-plane-manager` и нажмите на него;
   - Включите тумблер «Модуль включен».

### Настройка

Чтобы настроить модуль, используйте ModuleConfig/control-plane-manager и укажите необходимые параметры в `spec.settings`.

Пример с указанием версии схемы, включённым модулем и несколькими настройками:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  enabled: true
  settings:
    apiserver:
      bindToWildcard: true
      certSANs:
      - bakery.infra
      - devs.infra
      loadBalancer: {}
```

Как проверить, что control-plane-manager корректно запущен и не находится в состоянии ожидания, а также как проверить активные задачи (очереди) в Deckhouse:

1. Убедитесь, что модуль включён:

   ```console
   kubectl get modules control-plane-manager
   ```

1. Проверьте состояние подов control-plane-manager (поды находятся в пространстве имён `kube-system` и имеют лейбл `app=d8-control-plane-manager`):

   ```console
   kubectl -n kube-system get pods -l app=d8-control-plane-manager -o wide
   ```

   Убедитесь, что все поды находятся в статусе Running (или Completed).

1. Проверьте, что master-узлы в состоянии Ready:

   ```console
   kubectl get nodes -l node-role.kubernetes.io/control-plane
   ```

   Если требуется посмотреть более подробную информацию:

   ```console
   kubectl describe node <имя-узла>
   ```

1. Получите список очередей и активных заданий:

   ```console
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- \
    deckhouse-controller queue list
   ```

   Пример вывода:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

   > Перед выполнением «тяжёлых» процедур (например, перед переводом кластера из single-master в multi-master или перед обновлением версии Kubernetes) рекомендуется дождаться, чтобы все задачи в очередях были выполнены.

## Управление сертификатами

В DKP за выпуск и продление всех SSL-сертификатов компонентов control plane отвечает модуль `control-plane-manager`. Он контролирует:

1. **Серверные сертификаты** для kube-apiserver и etcd, хранящиеся в секрете `d8-pki` (пространство имён `kube-system`):
   - Корневой CA Kubernetes (`ca.crt`, `ca.key`);
   - Корневой CA etcd (`etcd/ca.crt`, `etcd/ca.key`);
   - RSA-сертификат и ключ для подписи Service Account’ов (`sa.pub`, `sa.key`);
   - Корневой CA для extension API-серверов (`front-proxy-ca.key`, `front-proxy-ca.crt`).

1. **Клиентские сертификаты**, необходимые для взаимного подключения компонентов control plane (например, `apiserver.crt`, `apiserver-etcd-client.crt` и т. д.). Эти файлы хранятся только на узлах. При любом изменении (например, добавлении новых SAN) сертификаты автоматически перевыпускаются, а kubeconfig синхронизируется.

### Управление PKI

DKP также управляет инфраструктурой приватных ключей (PKI), которая необходима для шифрования и аутентификации во всём кластере Kubernetes:

- PKI для компонентов control plane (kube-apiserver, kube-controller-manager, kube-scheduler и т. д.).
- PKI для кластера etcd (сертификаты etcd и межузлового взаимодействия).

DKP «забирает» управление этими PKI после завершения первоначальной установки кластера и запуска своих подов. Таким образом, все операции по выпуску, продлению и обновлению ключей (как для control plane, так и для etcd) выполняются автоматически и централизованно, без необходимости ручного вмешательства.

### Дополнительные SAN и автоматическое обновление

Deckhouse упрощает добавление новых точек входа (SAN) для API Kubernetes: достаточно прописать их в конфигурации. После любого изменения в SAN модуль автоматически регенерирует сертификаты и обновляет kubeconfig.

Если нужно добавить дополнительные SAN (дополнительные DNS-имена или IP-адреса) для API Kubernetes:

1. Пропишите новые SAN в `spec.settings.apiserver.certSANs` вашего ModuleConfig/control-plane-manager.
1. DKP автоматически сгенерирует новые сертификаты и обновит все необходимые конфигурационные файлы (включая kubeconfig).

### Ротация сертификатов kubelet

В Deckhouse Kubernetes Platform для kubelet не задают напрямую параметры `--tls-cert-file` и `--tls-private-key-file`. Вместо этого используется динамический сертификат:

- По умолчанию kubelet генерирует свои ключи в `/var/lib/kubelet/pki/` и при необходимости запрашивает продление у kube-apiserver;
- Срок действия выданных сертификатов — 1 год, но kubelet начинает обновление заранее (примерно за 5–10% оставшегося времени);
- Если обновить сертификат вовремя не удалось, узел помечается в статус `NotReady` и пересоздаётся.

### Ручное обновление сертификатов компонентов control plane

Если master-узлы долго не были доступны (к примеру, серверы были выключены), возможна ситуация, когда некоторые сертификаты в управляющем слое утрачивают актуальность. После включения узлов автоматического обновления не произойдёт — нужно выполнить процедуру вручную.

Обновление сертификатов компонентов управляющего слоя происходит с помощью утилиты `kubeadm`.
Чтобы обновить сертификаты, выполните следующие действия на каждом master-узле:

1. Найдите утилиту `kubeadm` на master-узле и создайте символьную ссылку c помощью следующей команды:

   ```shell
   ln -s  $(find /var/lib/containerd  -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

1. Выполните команду:

   ```shell
   kubeadm certs renew all
   ```

   Она пересоздаст нужные сертификаты (kube-apiserver, kube-controller-manager, kube-scheduler, etcd и т.д.).

## Масштабирование и переход single-master/multi-master

### Режимы работы control plane

Deckhouse Kubernetes Platform (DKP) поддерживает два режима работы control plane:

1. **Single-master**:
   - kube-apiserver использует только локальный экземпляр etcd.
   - На узле запускается прокси-сервер, который принимает запросы на `localhost`.
   - kube-apiserver "слушает" только на IP-адресе master-узла.

1. **Multi-master**:
   - kube-apiserver работает со всеми экземплярами etcd в кластере.
   - На всех узлах настраивается дополнительный прокси:
     - Если локальный kube-apiserver недоступен, запросы автоматически переадресуются к другим узлам.
   - Это обеспечивает отказоустойчивость и возможность масштабирования.

### Автоматическое масштабирование master-узлов

Deckhouse Kubernetes Platform (DKP) позволяет автоматически добавлять и удалять master-узлы, используя лейбл `node-role.kubernetes.io/control-plane=""`.

Автоматическое управление master-узлами:

- Добавление лейбла `node-role.kubernetes.io/control-plane=""` на узел:
  - Разворачиваются все компоненты control plane.
  - Узел подключается к etcd-кластеру.
  - Автоматически регенерируются сертификаты и конфигурационные файлы.

- Удаление лейбла `node-role.kubernetes.io/control-plane=""` с узла:
  - Компоненты control plane удаляются.
  - Узел корректно исключается из etcd-кластера.
  - Обновляются связанные конфигурационные файлы.

> **Важно.** Переход с 2 master-узлов до 1 требует ручной корректировки etcd. В остальных случаях изменение количества master-узлов выполняется автоматически.

### Типовые сценарии масштабирования

Deckhouse Kubernetes Platform (DKP) поддерживает автоматическое и ручное масштабирование master-узлов как в облачных, так и в bare-metal кластерах:

1. **Миграция single-master → multi-master**:

   - Добавьте один или несколько новых master-узлов.
   - Установите им лейбл `node-role.kubernetes.io/control-plane=""`.
   - DKP автоматически:
     - Развернёт все компоненты control plane.
     - Настроит узлы для работы с etcd-кластером.
     - Синхронизирует сертификаты и конфигурационные файлы.

1. **Миграция multi-master → single-master**:

   - Снимите лейблы `node-role.kubernetes.io/control-plane=""` и `node-role.kubernetes.io/master=""` со всех лишних master-узлов.
   - Для **bare-metal кластеров**:
     - Чтобы корректно исключить узлы из etcd:
       - Выполните команду `kubectl delete node <имя-узла>`;
       - Выключите соответствующие виртуальные машины или серверы.
         > **Важно:** в облачных кластерах все необходимые действия автоматически выполняются с помощью команды `dhctl converge`.

1. **Изменение числа master-узлов в облачном кластере**:

   - Аналогично добавлению/удалению узлов, но чаще всего выполняется с помощью команды `dhctl converge` или вручную через облачные инструменты.
     > **Важно.** Для стабильности кластера необходимо поддерживать нечётное число master-узлов для обеспечения кворума etcd.

### Как убрать роль master с узла, сохранив саму машину

Если необходимо вывести узел из состава master-узлов, но сохранить его в кластере для других задач, выполните следующие шаги:

1. Снимите лейблы, чтобы узел больше не рассматривался как master:

   ```bash
   kubectl label node <имя-узла> node-role.kubernetes.io/control-plane-
   kubectl label node <имя-узла> node-role.kubernetes.io/master-
   kubectl label node <имя-узла> node.deckhouse.io/group-
   ```

1. Удалите статические манифесты компонентов control plane, чтобы они больше не запускались на узле и лишние файлы PKI:

   ```bash
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Проверьте статус узла в etcd-кластере с помощью `etcdctl member list`.

   Пример:

   ```bash
   kubectl -n kube-system exec -ti \
   $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

После выполнения этих шагов узел больше не будет считаться master-узлом, но останется в кластере и может использоваться для других задач.

### Изменение образа ОС master-узлов в мультимастерном кластере

1. Сделайте резервную копию `etcd` и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет алертов, которые могут помешать обновлению master-узлов.
1. Убедитесь, что очередь Deckhouse пуста.
1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы проверить состояние перед началом работы:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Ответ должен сообщить, что Terraform не нашел расхождений и изменений не требуется.

1. **В контейнере с инсталлятором** выполните следующую команду и укажите необходимый образ ОС в параметре `masterNodeGroup.instanceClass` (укажите адреса всех master-узлов в параметре `--ssh-host`):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы провести обновление узлов:

   Внимательно изучите действия, которые планирует выполнить converge, когда запрашивает подтверждение.

   При выполнении команды узлы будут замены на новые с подтверждением на каждом узле. Замена будет выполняться по очереди в обратном порядке (2,1,0).

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Следующие действия (П. 9-12) **выполняйте поочередно на каждом** master-узле, начиная с узла с наивысшим номером (с суффиксом 2) и заканчивая узлом с наименьшим номером (с суффиксом 0).

1. **На созданном узле** откройте журнал systemd-юнита `bashible.service`. Дождитесь окончания настройки узла — в журнале должно появиться сообщение `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Проверьте, что узел etcd отобразился в списке узлов кластера:

   ```bash
   kubectl -n kube-system exec -ti \
   $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Убедитесь, что `control-plane-manager` функционирует на узле.

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Перейдите к обновлению следующего узла.

## Отказоустойчивость

Если какой-либо компонент control plane становится недоступным, кластер временно сохраняет текущее состояние, но не может обрабатывать новые события. Например:

- При сбое kube-controller-manager перестаёт работать масштабирование deployment'ов.
- При недоступности kube-apiserver невозможны любые запросы к Kubernetes API, но уже запущенные приложения продолжают функционировать.

Однако при продолжительной недоступности компонентов нарушается обработка новых объектов, реакция на сбои узлов и другие процессы. Через некоторое время это может повлиять и на пользователей.

Чтобы снизить такие риски, следует масштабировать control plane до отказоустойчивой конфигурации — минимум трёх узлов. Это особенно критично для etcd, так как он требует наличия кворума для выбора лидера. Кворум работает по принципу большинства (N/2 + 1) от общего числа узлов.

Пример:

| Размер кластера | Большинство | Максимальные потери |
|------------------|-------------|----------------------|
| 1                | 1           | 0                    |
| 3                | 2           | 1                    |
| 5                | 3           | 2                    |
| 7                | 4           | 3                    |
| 9                | 5           | 4                    |

> Обратите внимание: чётное число узлов не даёт преимущества по отказоустойчивости, но увеличивает накладные расходы на репликацию.

В большинстве случаев достаточно трёх узлов etcd. Пять — если критична устойчивость. Более семи — крайне редко и не рекомендуется из-за высокой нагрузки.

После добавления новых узлов control plane:

- Устанавливается лейбл `node-role.kubernetes.io/control-plane=""`.
- DaemonSet запускает поды на новых узлах.
- CPM создает или обновляет файлы в `/etc/kubernetes`: манифесты, конфигурации, сертификаты и т.д.
- Все модули DKP с поддержкой отказоустойчивости автоматически включают её, если значение глобальной настройки `highAvailability` не переопределено вручную.

Удаление узлов control plane выполняется в обратном порядке:

- Удаляются лейблы `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, `node.deckhouse.io/group`.
- CPM удаляет свои поды с этих узлов.
- Члены etcd, расположенные на этих узлах, удаляются автоматически.
- Если число узлов уменьшается с двух до одного, etcd может перейти в статус `readonly`. В этом случае требуется запуск с параметром `--force-new-cluster`, который следует убрать после успешного запуска.

## Обновление и управление версиями

Процесс обновления control plane в DKP полностью автоматизирован.

- В DKP поддерживаются последние пять версий Kubernetes.
- Control plane можно откатывать на одну минорную версию назад и обновлять на несколько версий вперёд — шаг за шагом, по одной версии за раз.
- Patch-версии (например, 1.27.3 → 1.27.5) обновляются автоматически вместе с версией Deckhouse, и управлять этим процессом нельзя.
- Minor-версии задаются вручную в параметре `kubernetesVersion` в ресурсе ClusterConfiguration.

### Как изменить версию Kubernetes

1. Откройте редактирование ClusterConfiguration:

   ```console
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller edit cluster-configuration
   ```

1. Установите желаемую версию Kubernetes (`kubernetesVersion`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   cloud:
     prefix: demo-stand
     provider: Yandex
   clusterDomain: cloud.education
   clusterType: Cloud
   defaultCRI: Containerd
   kubernetesVersion: "1.30"
   podSubnetCIDR: 10.111.0.0/16
   podSubnetNodeCIDRPrefix: "24"
   serviceSubnetCIDR: 10.222.0.0/16
   ```

1. Сохраните изменения.

## Восстановление etcd

### Просмотр списка узлов кластера в etcd

Ниже приведены шаги для просмотра списка узлов, которые состоят в etcd-кластере:

1. Найдите под etcd:

   ```console
   kubectl -n kube-system get pods -l component=etcd,tier=control-plane
   ```

   Обычно имя пода содержит префикс `etcd-`.

1. Выполните команду на любом доступном etcd-поде (предполагается, что он запущен в пространстве имён `kube-system`):

   ```console
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
     etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
   ```

   В данной команде используется подстановка: `$(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1)`. Она автоматически подставит имя первого пода, соответствующего нужным лейблам.

### Если etcd не функционирует

1. Остановите все узлы etcd, кроме одного, удалив манифест `etcd.yaml` на остальных.
1. На оставшемся узле добавьте в команду etcd параметр `--force-new-cluster`.
1. После восстановления удалите этот флаг. 
   > **Будьте осторожны**: это полностью уничтожает предыдущие данные и формирует новый кластер etcd.

### Если etcd постоянно перезапускается c ошибкой вида panic: unexpected removal of unknown remote peer

В некоторых случаях помогает ручное восстановление через `etcdutl snapshot restore`:

1. Сохраните локальный снапшот `/var/lib/etcd/member/snap/db`.
1. Воспользуйтесь `etcdutl` с опцией `--force-new-cluster`.
1. Полностью очистите `/var/lib/etcd` и положите туда восстановленный снапшот.
1. Удалите «зависшие» контейнеры etcd / kube-apiserver, перезапустите узел.

### Как ускорить перезапуск подов при потере связи с узлом

1. Сократите время перехода узла в статус `Unreachable`. Измените параметр `nodeMonitorGracePeriodSeconds` (по умолчанию 40 секунд). Например, установите 10 секунд.
1. Ускорьте высвобождение подов с недоступного узла. Измените параметр `failedNodePodEvictionTimeoutSeconds` (по умолчанию 300 секунд). Например, установите 50 секунд.

> **Важно**. Чем короче таймауты, тем чаще системные компоненты проверяют состояние узлов и планируют перемещение подов. Это повышает нагрузку на control plane, поэтому выбирайте значения, соответствующие вашим требованиям к отказоустойчивости и производительности.
