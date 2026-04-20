---
title: Что делать, если после добавления объекта не создаются порождаемые им ресурсы?
subsystems:
  - cluster_infrastructure
lang: ru
---

Если после создания объекта в системе (например, [dexAuthenticator](/modules/user-authn/cr.html#dexauthenticator)) нужные ресурсы не появились, выполните следующие шаги:

1. Проверьте, есть ли в кластере критические алерты, которые могут блокировать создание нужных объектов. Для этого используйте команду:

   ```shell
   d8 k get clusteralerts.deckhouse.io
   ```

   Пример вывода:

   ```console
   NAME               ALERT                                           SEVERITY   AGE   LAST RECEIVED   STATUS
   012f602592aa7a91   K8SSchedulerTargetDown                          3          16h   54s             firing
   0836dc893d5ecc65   KubernetesDeploymentReplicasUnavailable         5          15h   62s             firing
   08742f87d62d0063   NTPDaemonOnNodeDoesNotSynchronizeTime           5          16h   46s             firing
   172cfd38d2f7fd19   D8DeckhouseQueueIsHung                          7          12h   66s             firing
   1c5705daf731f5cf   D8StrongholdNoActiveNodes                       3          16h   55s             firing
   1d2c2f7d69f69f4b   D8DeckhouseIsNotOnReleaseChannel                9          12h   53s             firing
   205a551243d795f3   D8LogShipperAgentNotScheduledInCluster          7          15h   63s             firing
   2e34039aa7a3018e   D8NodeIsNotUpdating                             9          12h   47s             firing
   31baf9a70d657275   D8StrongholdClusterNotHealthy                   7          16h   55s             firing
   ```

   <!-- TODO: заменить ссылку на относительную перед публикацией FAQ на сайте с документацией -->

   Подробнее об алертах — в разделе [«Список алертов»](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/alerts.html).

1. Проверьте очередь заданий Deckhouse:

   ```shell
   d8 s queue list
   ```

   Пример вывода (очереди пусты):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

   Если в очереди много необработанных или долго выполняющихся заданий, это может говорить о проблемах.

1. Проанализируйте логи и события DKP:

   - Для просмотра логов в реальном времени используйте команду:

     ```shell
     d8 k -n d8-system logs -f -l app=deckhouse
     ```

     Пример вывода:

     ```console
     {"level":"info","logger":"addon-operator","msg":"ConvergeModules task for OperatorStartup in phase '', trigger is Operator-Startup","binding":"ConvergeModules","event.type":"OperatorStartup","queue":"main","task.flow":"start","task.id":"fde0eb3b-5c3e-4da6-a0d8-a52f8ae03428","time":"2025-11-26T08:29:33Z"}
     {"level":"warn","logger":"addon-operator.converge-modules","msg":"ConvergeModules: functional scheduler not finished","binding":"ConvergeModules","event.type":"OperatorStartup","queue":"main","task.id":"fde0eb3b-5c3e-4da6-a0d8-a52f8ae03428","time":"2025-11-26T08:29:33Z"}
     ```

     При анализе логов особое внимание обращайте на предупреждения (`WARNING`) и сообщения об ошибках (`ERROR`).

   - Для просмотра событий используйте команду:

     ```shell
     d8 k -n d8-system get events
     ```

     Пример вывода:

     ```console
     LAST SEEN   TYPE      REASON              OBJECT                                          MESSAGE
     11m         Warning   Unhealthy           pod/deckhouse-5886c9bd77-vgdbw                  Readiness probe failed: HTTP probe failed with statuscode: 500
     7m22s       Normal    SuccessfulDelete    replicaset/deckhouse-5886c9bd77                 Deleted pod: deckhouse-5886c9bd77-vgdbw
     7m20s       Normal    Scheduled           pod/deckhouse-6bc5c4494-fwx6z                   Successfully assigned d8-system/deckhouse-6bc5c4494-fwx6z to sandbox1-master-0
     7m20s       Normal    Pulling             pod/deckhouse-6bc5c4494-fwx6z                   Pulling image "dev-registry.deckhouse.io/sys/deckhouse-oss@sha256:17ac07634e17422df52720264cddec3916ed6985a77782dc8a24fe5352290e6e"
     ```

    При анализе событий особое внимание обращайте на те, у которых тип `Warning`.
