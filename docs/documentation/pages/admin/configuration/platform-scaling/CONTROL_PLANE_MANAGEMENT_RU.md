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

1. Включение через UI (TODO).

### Общая схема настройки

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

## Управление сертификатами

В DKP за выпуск и продление всех SSL-сертификатов компонентов control plane отвечает модуль `control-plane-manager`. Он контролирует:

1. **Серверные сертификаты** для kube-apiserver и etcd, хранящиеся в секрете `d8-pki` (пространство имён `kube-system`):
   - Корневой CA Kubernetes (`ca.crt`, `ca.key`);
   - Корневой CA etcd (`etcd/ca.crt`, `etcd/ca.key`);
   - RSA-сертификат и ключ для подписи Service Account’ов (`sa.pub`, `sa.key`);
   - Корневой CA для extension API-серверов (`front-proxy-ca.key`, `front-proxy-ca.crt`).

1. **Клиентские сертификаты**, необходимые для взаимного подключения компонентов control plane (например, `apiserver.crt`, `apiserver-etcd-client.crt` и т. д.). Эти файлы хранятся только на узлах. При любом изменении (например, добавлении новых SAN) сертификаты автоматически перевыпускаются, а kubeconfig синхронизируется.

### Дополнительные SAN и автоматическое обновление

Deckhouse упрощает добавление новых точек входа (SAN) для API Kubernetes: достаточно прописать их в конфигурации. После любого изменения в SAN модуль автоматически регенерирует сертификаты и обновляет kubeconfig.

### Ротация сертификатов kubelet

В Deckhouse Kubernetes Platform для kubelet*не задают напрямую `--tls-cert-file` и `--tls-private-key-file`. Вместо этого используется динамический сертификат:

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

1. Обновите сертификаты:

   ```shell
   kubeadm certs renew all
   ```

## Масштабирование и переход single-master/multi-master

### Режимы работы control plane

Deckhouse Kubernetes Platform (DKP) поддерживает два режима работы control plane:

1. **Single-master**:
   - kube-apiserver использует только локальный экземпляр etcd.
   - На узле запускается прокси-сервер, который принимает запросы на `localhost`.
   - kube-apiserver слушает только на IP-адресе master-узла.

2. **Multi-master**:
   - kube-apiserver работает со всеми экземплярами etcd в кластере.
   - На каждом master-узле настраивается дополнительный прокси:
     - Если локальный kube-apiserver недоступен, запросы автоматически переадресуются к другим узлам.
   - Это обеспечивает отказоустойчивость и возможность масштабирования.

### Масштабирование master-узлов

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

Deckhouse Kubernetes Platform (DKP) поддерживает автоматическое и ручное масштабирование master-узлов.

1. **Миграция single-master → multi-master**:

- Добавьте один или несколько новых master-узлов.
- Установите им лейбл `node-role.kubernetes.io/control-plane=""`.
- DKP автоматически:
  - Развернёт все компоненты control plane.
  - Настроит узлы для работы с etcd-кластером.
  - Синхронизирует сертификаты и конфигурационные файлы.

1. **Миграция multi-master → single-master**:

- Снимите лейблы `node-role.kubernetes.io/control-plane=""` и `node-role.kubernetes.io/master=""` со всех "лишних" master-узлов.
- Для корректного исключения узлов из etcd:
  - Выполните команду `kubectl delete node <имя-узла>`.
  - Выключите соответствующие виртуальные машины или серверы.

1. **Изменение числа master-узлов в облачном кластере**:

- Аналогично добавлению/удалению master-узлов, но часто выполняется через `dhctl` или вручную.
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

1. Проверьте статус узла в etcd-кластере:

   ```bash
   etcdctl member list
   ```

После выполнения этих шагов узел больше не будет считаться master-узлом, но останется в кластере и может использоваться для других задач.

### Изменение образа ОС master-узлов в мультимастерном кластере

1. Сделайтерезервную копию `etcd` и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере неталертов, которые могут помешать обновлению master-узлов.
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

### Как изменить образ ОС в кластере с одним master-узлом

1. Преобразуйте кластер с одним master-узлом в мультимастерный в соответствии с [инструкцией](#todo).
1. Обновите master-узлы в соответствии с [инструкцией](#todo).
1. Преобразуйте мультимастерный кластер в кластер с одним master-узлом в соответствии с [инструкцией](#todo)

### Добавление и удаление master-узлов

Как добавить master-узел в статическом, гибридном или облачном кластере?

Просто поставьте лейбл node-role.kubernetes.io/control-plane="". Если это облачный кластер, чаще всего обновляют provider-cluster-configuration и запускают dhctl converge. Важно иметь нечётное число master-узлов для кворума.

Как убрать роль master-узла, сохранив узел?

Снимите лейблы node-role.kubernetes.io/control-plane="" и node-role.kubernetes.io/master="", удалите манифесты /etc/kubernetes/manifests/{etcd,kube-apiserver,...}.yaml, лишние файлы конфигурации и перезапустите узел.

## Управление версиями (upgrade/downgrade)

1. Patch-версии (например, 1.27.3 → 1.27.5) обновляются автоматически вместе с версией Deckhouse. Управлять этим процессом нельзя.

1. Minor-версии (например, 1.26.* → 1.28.*) задаются параметром kubernetesVersion. Можно выбрать:

   - Automatic: обновления будут происходить автоматически до максимально поддерживаемой версии.
   - Указать конкретную минорную версию.

Обновление Control Plane выполняется безопасно и поддерживает сценарии:

- Upgrade с «шагом» в одну минорную версию (1.26 → 1.27 → 1.28 и т.д.).
- Downgrade на одну минорную версию от максимальной, когда-либо использовавшейся. Сначала понижается kubelet, затем компоненты Control Plane.

Во время обновлений возможна кратковременная недоступность API, но на работу приложений в кластере это не влияет.

## Восстановление etcd

### Просмотр списка узлов кластера в etcd

```bash
kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
  etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ member list -w table
```

### Если etcd не функционирует

1. Остановите все узлы etcd, кроме одного, удалив манифест etcd.yaml на остальных.
1. На оставшемся узле добавьте в команду etcd параметр --force-new-cluster.
1. После восстановления удалите этот флаг. Будьте осторожны: это полностью уничтожает предыдущие данные и формирует новый кластер etcd.

### Если etcd постоянно перезапускается c ошибкой вида panic: unexpected removal of unknown remote peer

В некоторых случаях помогает ручное восстановление через etcdutl snapshot restore:

1. Сохраните локальный снапшот /var/lib/etcd/member/snap/db.
1. Воспользуйтесь etcdutl с опцией --force-new-cluster.
1. Полностью очистите /var/lib/etcd и положите туда восстановленный снапшот.
1. Удалите «зависшие» контейнеры etcd / kube-apiserver, перезапустите узел

### Как ускорить перезапуск подов при потере связи с узлом

По умолчанию узел становится Unreachable через 40 секунд, а поды с него «снимаются» ещё через 5 минут. Если нужно ускорить:

- nodeMonitorGracePeriodSeconds: уменьшить до, скажем, 10 секунд.
- failedNodePodEvictionTimeoutSeconds: уменьшить до 50 секунд.

Но учитывайте, что более агрессивные таймауты повышают нагрузку на Control Plane.

## Примеры процедур (изменение числа master-узлов и т. д.)
