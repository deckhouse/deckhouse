---
title: "Основы управления узлами в Deckhouse"
permalink: ru/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/node-management.html
lang: ru
---

Deckhouse Virtualization Platform (DVP) поддерживает полный цикл управления узлами:

- Автоматическое масштабирование количества узлов в зависимости от нагрузки;
- Обновление узлов и поддержание их в актуальном состоянии;
- Централизованное управление конфигурацией групп узлов через CRD NodeGroup;
- Использование различных типов узлов: постоянные, временные или bare-metal.

Группы узлов позволяют логически сегментировать инфраструктуру кластера. В DVP часто используются следующие типы [NodeGroup](/modules/node-manager/cr.html#nodegroup) по назначению:

- `master` — управляющие узлы (control plane);
- `front` — узлы для маршрутизации HTTP(S)-трафика;
- `monitoring` — узлы для размещения компонентов мониторинга;
- `worker` — узлы для пользовательских приложений;
- `system` — выделенные узлы для системных компонентов.

В каждой группе можно централизованно задавать настройки узлов, включая версию Kubernetes, ресурсы, taint'ы, лейблы, параметры kubelet и прочее.

## Включение механизма управления узлами

Управление узлами реализовано с помощью модуля [`node-manager`](/modules/node-manager/), который можно включить или выключить несколькими способами:

1. Через ресурс ModuleConfig/node-manager:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: node-manager
   spec:
     version: 2
     enabled: true
     settings:
       earlyOomEnabled: true
       instancePrefix: kube
       mcmEmergencyBrake: false
   ```

1. Командой:

   ```shell
   d8 system module enable node-manager
   # Или disable.
   ```

1. Через [веб-интерфейс Deckhouse](/modules/console/):

   - Перейдите в раздел «Deckhouse - «Модули»;
   - Найдите модуль `node-manager` и нажмите на него;
   - Включите тумблер «Модуль включен».

## Автоматическое развёртывание и обновление

В Deckhouse Virtualization Platform (DVP) реализован автоматизированный механизм управления жизненным циклом узлов на основе объектов [NodeGroup](/modules/node-manager/cr.html#nodegroup). DVP обеспечивает как начальное развёртывание узлов, так и их обновление при изменении конфигурации, поддерживая bare-metal кластеры (при наличии интеграции с модулем `node-manager`).

Как это работает:

1. NodeGroup — основной объект управления группами узлов. Он определяет тип узлов, их количество, шаблоны ресурсов и ключевые параметры (например, настройки kubelet, taint'ов и др.).
1. При создании или изменении NodeGroup, модуль `node-manager` автоматически приводит состояние узлов в соответствие с заданной конфигурацией.
1. Обновление происходит без вмешательства пользователя — устаревшие узлы удаляются, новые создаются автоматически.

Рассмотрим автоматическое обновление на примере обновления версии kubelet.

1. Пользователь изменяет параметры в секции `kubelet` объекта NodeGroup.
1. DVP определяет, что текущие узлы не соответствуют новой конфигурации.
1. Последовательно создаются новые узлы с обновлёнными настройками.
1. Старые узлы постепенно удаляются из кластера.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-cloud
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: AnotherCloudInstanceClass
         name: my-class
   ```

## Базовая настройка узлов и операционной системы

При создании и подключении узлов DVP автоматически выполняет ряд действий, необходимых для корректной работы кластера:

- установка и настройка поддерживаемой операционной системы;
- отключение автоматических обновлений пакетов;
- настройка журналирования и системных параметров;
- установка необходимых пакетов и утилит;
- настройка компонента `nginx` для балансировки трафика от `kubelet` к API-серверам;
- установка и конфигурация компонентов container runtime (`containerd`) и `kubelet`;
- включение узла в состав кластера Kubernetes.

Эти операции выполняются автоматически при использовании `bootstrap.sh` или при подключении узлов через ресурсы [StaticInstance](/modules/node-manager/cr.html#staticinstance) и [SSHCredentials](/modules/node-manager/cr.html#sshcredentials).

### Обновления, требующие прерывания работы узла

Некоторые обновления, например, обновление версии `containerd` или обновление kubelet на несколько версий вперед,
требуют прерывания работы узла и могут привести к кратковременному простою системных компонентов (*disruptive-обновления*).
Режим применения таких обновлений настраивается с помощью параметра `disruptions.approvalMode`:

- `Manual` — режим ручного подтверждения disruptive-обновлений.
  При появлении доступного disruptive-обновления отображается [алерт `NodeRequiresDisruptionApprovalForUpdate`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#node-manager-noderequiresdisruptionapprovalforupdate).
  
  Чтобы подтвердить disruptive-обновление,
  установите аннотацию `update.node.deckhouse.io/disruption-approved=` на каждый узел в группе, следуя примеру:

  ```shell
  d8 k annotate node ${NODE_1} update.node.deckhouse.io/disruption-approved=
  ```

  > **Важно**. В этом режиме не выполняется автоматический drain узла.
  > При необходимости выполните drain вручную перед установкой аннотации.
  >
  > Чтобы избежать проблем при выполнении drain,
  > всегда устанавливайте режим `Manual` для группы master-узлов.

- `Automatic` — режим автоматического разрешения disruptive-обновлений.
  
  В этом режиме по умолчанию выполняется автоматический drain узла перед применением обновления.
  Поведение можно изменить с помощью параметра `disruptions.automatic.drainBeforeApproval` в настройках узла.

- `RollingUpdate` — режим, при котором будет создан новый узел с обновлёнными настройками, а старый будет удалён.

  В этом режиме на время обновления в кластере создаётся дополнительный узел.
  Это может быть удобно, если в кластере нет ресурсов для временного размещения нагрузки с обновляемого узла.

## Пример системной NodeGroup

Системные узлы — это узлы, предназначенные для запуска системных компонентов. Обычно они выделяются с помощью лейблов и taint'ов, чтобы туда не попадали пользовательские поды.
Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: Static
```

## Примеры описания NodeGroupConfiguration

### Установка плагина cert-manager для kubectl на master-узлах

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-cert-manager-plugin.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "master"
  content: |
    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/kubectl-cert_manager-linux-amd64.tar.gz -o - | tar -zxvf - kubectl-cert_manager
    mv kubectl-cert_manager /usr/local/bin
```

## Конфигурация werf для игнорирования состояния Ready у группы узлов

[Werf](https://ru.werf.io) проверяет состояние `Ready` у ресурсов и в случае его наличия дожидается, пока значение станет `True`.

Создание (обновление) ресурса NodeGroup в кластере может занять значительное время (до готовности всех узлов). При использовании werf (например, в CI/CD) это может привести к превышению таймаута сборки.

Чтобы werf игнорировал состояние NodeGroup, добавьте к ресурсу [NodeGroup](/modules/node-manager/cr.html#nodegroup) следующие аннотации:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

## Настройки для групп с узлами Static

Группы узлов с типами Static предназначены для управления вручную созданными узлами. Эти узлы подключаются вручную или через [StaticInstance](/modules/node-manager/cr.html#staticinstance) и не поддерживают автоматическое обновление и масштабирование.

Особенности конфигурации:

- Все действия по обновлению (обновление kubelet, перезапуск, замена узлов) выполняются вручную или через внешние автоматизации вне DVP.

- Рекомендуется явно указывать желаемую версию kubelet, чтобы обеспечить единообразие между узлами, особенно если они подключаются с разными версиями вручную:
  
  ```yaml
  nodeTemplate:
     kubelet:
       version: "1.28"
  ```

- Подключение узлов к кластеру может выполняться вручную или автоматически, в зависимости от конфигурации:
  - **Вручную** — пользователь скачивает bootstrap-скрипт, настраивает сервер, запускает скрипт вручную.
  - **Автоматически (CAPS)** — при использовании [StaticInstance](/modules/node-manager/cr.html#staticinstance) и [SSHCredentials](/modules/node-manager/cr.html#sshcredentials), DVP автоматически подключает и настраивает узлы.
  - **Смешанный подход** — вручную добавленный узел можно передать под управление CAPS, используя аннотацию `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

Если включён Cluster API Provider Static (CAPS), в NodeGroup можно использовать секцию `staticInstances`. Это позволяет DVP автоматически подключать, настраивать и, при необходимости, отключать статические узлы на основе ресурсов StaticInstance и SSHCredentials.

> В [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типами Static можно явно указать количество узлов в параметре `spec.staticInstances.count`. Это позволяет задать ожидаемое количество узлов — DVP использует это значение для контроля состояния и автоматизации.

## Запуск DVP на произвольном узле

Для запуска DVP на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](/modules/deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`. Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

{% alert level="warning" %}
Используйте для запуска DVP только узлы с типом **Static**. Также избегайте использования для запуска DVP группы узлов (`NodeGroup`), содержащей только один узел.
{% endalert %}

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    nodeSelector:
      node-role.deckhouse.io/deckhouse: ""
```
