---
title: Что делать при наличии проблем с обновлением компонентов Kubernetes на узлах кластера, с синхронизацией узлов, применением NodeGroup Configuration?
permalink: ru/faq-common/update-kubernetes-components-problems.html
lang: ru
---

Если на узле кластера не обновляются компоненты Kubernetes, не применяется конфигурация [NodeGroup](/modules/node-manager/cr.html#nodegroup), не все узлы [NodeGroup](/modules/node-manager/cr.html#nodegroup) синхронизированы (имеют статус `UPTODATE`), выполните следующие шаги:

1. Проверьте логи bashible на узле, на котором имеются проблемы. Механизм bashible используется для поддержания узлов кластера в актуальном состоянии. Он запускается таймером `bashible.timer` с определенной периодичностью как сервис на узлах кластера. При этом происходит перезапуск, синхронизация скриптов и их выполнение (при необходимости).

   Для проверки логов bashible используйте команду:

   ```shell
   journalctl -u bashible
   ```

   Если в ответе содержится запись `Configuration is in sync, nothing to do`, узел синхронизирован и проблем нет. Отсутствие этой записи или наличие ошибок указывает на проблему.

1. Проверьте состояние синхронизации узлов кластера с помощью команды:

   ```shell
   d8 k get ng
   ```

   Количество узлов в статусе `UPTODATE` должно совпадать с общим количеством узлов в каждой группе.

   Пример вывода:

   ```console
   NAME       TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   frontend   Static   1       1       1                                                               118d   True
   master     Static   3       3       3                                                               118d   True
   system     Static   2       2       2                                                               118d   True
   worker     Static   2       2       2                                                               118d   True
   ```
