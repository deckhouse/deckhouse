---
title: Переключение CNI в кластере
permalink: ru/guides/cni-migration.html
description: Инструкция по переключению (миграции) CNI в кластере Deckhouse.
lang: ru
layout: sidebar-guides
---

Документ описывает процедуру смены сетевого плагина (CNI) в кластере Deckhouse Kubernetes Platform. Используемый в DKP инструмент позволяет выполнить автоматизированную миграцию (например, с Flannel на Cilium) с минимальным простоем приложений и без полной перезагрузки узлов кластера.

{% alert level="danger" %}

* Инструмент не предназначен для переключения на любой (сторонний) CNI.
* В процессе миграции автоматически будет включен модуль целевого CNI (`ModuleConfig.spec.enabled: true`), который предварительно должен быть настроен администратором.

{% endalert %}

{% alert level="warning" %}

* В процессе миграции произойдет перезапуск всех подов в кластере, использующих сеть (в PodNetwork), созданную текущим CNI. Это вызовет перерыв в доступности сервисов. С целью минимизации рисков потери критичных данных рекомендуется перед проведением работ остановить работу наиболее критичных прикладных сервисов самостоятельно.
* Рекомендуется проводить работы в согласованное технологическое окно.
* Перед проведением работ необходимо отключить внешние системы управления кластером (CI/CD, GitOps, ArgoCD и т.д.), которые могут конфликтовать с процессом (например, пытаться восстановить удаленные поды раньше времени или откатывать настройки). Также, необходимо убедиться, что система управления кластером не включит старый модуль CNI.

{% endalert %}

Переключение CNI в кластере DKP можно выполнить несколькими способами.

## Способ 1: Использование группы команд d8 cni-migration утилиты d8 (автоматизированное переключение)

Утилита [d8](/products/kubernetes-platform/documentation/v1/cli/d8/reference/) предоставляет группу команд `d8 cni-migration` для управления процессом миграции.

### Запуск миграции

Для начала процесса выполните команду `switch`, указав целевой CNI (например, `cilium`, `flannel` или `simple-bridge`):

```bash
d8 cni-migration switch --to-cni cilium
```

Эта команда создаст необходимый ресурс в кластере и запустит контроллер миграции. DKP автоматически развернет необходимые компоненты: Менеджер (Manager) и Агенты (Agents) в неймспейсе `d8-system`.

### Наблюдение за прогрессом

Чтобы следить за ходом выполнения в реальном времени, используйте команду:

```bash
d8 cni-migration watch
```

Вы увидите динамический интерфейс со следующей информацией:

* **Текущая фаза** — что именно происходит в данный момент (например, `CleaningNodes` или `RestartingPods`).
* **Прогресс** — список успешно завершенных этапов и текущий статус ожидания действий в кластере.
* **Ошибки** — если на каком-то узле возникнет проблема, она будет отображена в списке `Failed Nodes`.

Основные фазы процесса:

1. **Preparing** — проверка запроса и ожидание готовности среды (например, отключение вебхуков).
2. **WaitingForAgents** — ожидание запуска агентов миграции на всех узлах.
3. **EnablingTargetCNI** — включение модуля целевого CNI в конфигурации Deckhouse.
4. **DisablingCurrentCNI** — выключение модуля текущего CNI.
5. **CleaningNodes** — очистка сетевых настроек текущего CNI агентами на узлах.
6. **WaitingTargetCNI** — ожидание готовности подов нового CNI (DaemonSet).
7. **RestartingPods** — перезапуск прикладных подов для переключения их на новую сеть.
8. **Completed** — миграция успешно завершена.

### Завершение и очистка

После того как статус миграции перейдет в `Succeeded`, удалите ресурсы миграции (контроллеры и агенты), чтобы они не потребляли ресурсы кластера. Для этого используйте команду:

```bash
d8 cni-migration cleanup
```

## Способ 2: Использование команд d8 k (ручное переключение)

Управлять миграцией можно напрямую через Kubernetes API.

### Запуск миграции

Создайте ресурс [CNIMigration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#cnimigration) (в примере используется манифест `cni-migration.yaml`) с указанием целевого CNI:

```yaml
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-to-cilium
spec:
  targetCNI: cilium
```

Примените манифест в кластере:

```bash
d8 k create -f cni-migration.yaml
```

### Наблюдение за прогрессом

Отслеживайте статус ресурса CNIMigration:

```bash
d8 k get cnimigration migration-to-cilium -o yaml -w
# или
watch -n 1 "d8 k get cnimigration migration-to-cilium -o yaml"
```

Обращайте внимание на поля:

* `status.phase` — текущий этап.
* `status.conditions` — детальная история переходов.
* `status.failedSummary` — список узлов с ошибками.

Для детальной диагностики конкретного узла можно проверить его локальный ресурс:

```bash
d8 k get cninodemigrations
d8 k get cninodemigration NODE_NAME -o yaml
```

Для просмотра логов контроллеров миграции в кластере выполните следующие команды:

```bash
d8 k -n d8-system get pods -o wide | grep cni-migration
d8 k -n d8-system logs cni-migration-manager-HASH
d8 k -n d8-system logs cni-migration-agent-HASH
```

### Завершение и очистка

После успешного завершения (в статусе ресурса CNIMigration появится условие `Type: Succeeded, Status: True`) удалите ресурс:

```bash
d8 k delete cnimigration migration-to-cilium
```

Это действие даст DKP сигнал удалить все ранее созданные ресурсы в кластере.

{% offtopic title="Устаревший способ переключения CNI c Flannel на Cilium..." %}

1. Выключите [модуль `kube-proxy`](/modules/kube-proxy/):

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-proxy
   spec:
     enabled: false
   EOF
   ```

1. Включите [модуль `cni-cilium`](/modules/cni-cilium/):

   ```shell
   d8 k create -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cni-cilium
   spec:
     version: 1
     enabled: true
     settings:
     tunnelMode: VXLAN
   EOF
   ```

1. Убедитесь, что все агенты Cilium перешли в статус `Running`:

   ```shell
   d8 k get po -n d8-cni-cilium
   ```

   Пример вывода:

   ```console
   NAME                      READY STATUS  RESTARTS    AGE
   agent-5zzfv               2/2   Running 5 (23m ago) 26m
   agent-gqb2b               2/2   Running 5 (23m ago) 26m
   agent-wtv4p               2/2   Running 5 (23m ago) 26m
   operator-856d69fd49-mlglv 2/2   Running 0           26m
   safe-agent-updater-26qpk  3/3   Running 0           26m
   safe-agent-updater-qlbrh  3/3   Running 0           26m
   safe-agent-updater-wjjr5  3/3   Running 0           26m
   ```

1. Перезагрузите master-узлы.

1. Перезагрузите остальные узлы кластера.

   > Если агенты Cilium не переходят в статус `Running`, перезагрузите проблемные узлы.

1. Выключите [модуль `cni-flannel`](/modules/cni-flannel/):

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cni-flannel
   spec:
     enabled: false
   EOF
   ```

1. Включите [модуль `node-local-dns`](/modules/node-local-dns/):

   ```shell
   d8 k apply -f - << EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: node-local-dns
   spec:
     enabled: true
   EOF
   ```

   После включения модуля дождитесь перехода всех агентов Cilium в состояние `Running`.

1. Убедитесь, что переключение CNI с Flannel на Cilium прошло успешно.

Чтобы убедиться в том, что переключение CNI с Flannel на Cilium прошло успешно:

1. Проверьте очередь Deckhouse.

   * В случае одного master-узла:

     ```shell
     d8 system queue list
     ```

   * В случае мультимастерной инсталляции:

     ```shell
     d8 system queue list
     ```

1. Проверьте агенты Cilium. Они должны быть в статусе `Running`:

   ```shell
   d8 k get po -n d8-cni-cilium
   ```

   Пример вывода:

   ```console
   NAME        READY STATUS  RESTARTS    AGE
   agent-5zzfv 2/2   Running 5 (23m ago) 26m
   agent-gqb2b 2/2   Running 5 (23m ago) 26m
   agent-wtv4p 2/2   Running 5 (23m ago) 26m
   ```

1. Проверьте, что модуль `cni-flannel` выключен:

   ```shell
   d8 k get modules | grep flannel
   ```

   Пример вывода:

   ```console
   cni-flannel                         35     Disabled    Embedded
   ```

1. Проверьте, что модуль `node-local-dns` включен:

   ```shell
   d8 k get modules | grep node-local-dns
   ```

   Пример вывода:

   ```console
   node-local-dns                      350    Enabled     Embedded     Ready
   ```

{% endofftopic %}

## Устранение неполадок

{% alert %}

Инструмент переключения CNI не оценивает сетевую связанность подов и компонентов кластера после миграции CNI в кластере.

{% endalert %}

### Агент не запускается на узле

Проверьте статус DaemonSet `cni-migration-agent` в неймспейсе `d8-system`. Возможно, на узле есть taints, которые не покрыты tolerations агента.

### Узел застрял в фазе CleaningNodes

Проверьте логи пода агента на соответствующем узле:

```bash
d8 k -n d8-system logs cni-migration-agent-HASH
```

Возможная причина: невозможность удалить файлы конфигурации CNI из-за прав доступа, зависших процессов, невозможности пройти процедуру проверки вебхуков.

### Поды целевого CNI не стартуют

Если целевой CNI (например, Cilium) находится в статусе `Init:0/1`, проверьте логи его init-контейнера `cni-migration-init-checker`. Он ожидает завершения очистки узла. Если очистка не завершена (см. пункт выше), новый CNI не запустится. В критической ситуации можно отредактировать DaemonSet с целью удаления init-контейнера `cni-migration-init-checker`.

### Миграция зависла

Если процесс остановился и не продвигается долгое время:

1. Проверьте `failedSummary` в статусе CNIMigration.
1. Если есть проблемные узлы, работу которых невозможно исправить (например, узел в статусе NotReady), можете временно удалить этот узел из кластера или попробовать его перезагрузить.
