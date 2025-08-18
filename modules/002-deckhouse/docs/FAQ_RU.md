---
title: "Модуль deckhouse: FAQ"
---

## Как запустить kube-bench в кластере?

1. Зайдите внутрь пода Deckhouse:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- bash
   ```

1. Выберите, на каком узле запустить kube-bench.

   * Запуск на случайном узле:

     ```shell
     curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
     ```

   * Запуск на конкретном узле, например на control-plane:

     ```shell
     curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl apply -f - --dry-run=client -o json | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
     ```

1. Проверьте результат выполнения:

   ```shell
   kubectl logs job.batch/kube-bench
   ```

{% alert level="warning" %}
В Deckhouse установлен срок хранения логов — 7 дней. Однако, в соответствии с требованиями безопасности указанными в kube-bench, логи должны храниться не менее 30 дней. Используйте отдельное хранилище для логов, если вам необходимо хранить логи более 7 дней.
{% endalert %}

## Как собрать информацию для отладки?

Мы всегда рады помочь пользователям с расследованием сложных проблем. Пожалуйста, выполните следующие шаги, чтобы мы смогли вам помочь:

1. Выполните следующую команду, чтобы собрать необходимые данные:

   ```sh
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Отправьте получившийся архив [команде Deckhouse](https://github.com/deckhouse/deckhouse/issues/new/choose) для дальнейшего расследования.

Данные, которые будут собраны:

##### Deckhouse:
* Состояние очереди Deckhouse;
* Deckhouse values. За исключением значений `kubeRBACProxyCA` и `registry.dockercfg`;
* Данные о текущей версии пода deckhouse;
* Все объекты `DeckhouseRelease`;
* Логи подов Deckhouse;
* Манифесты controller'ов и подов из всех пространств имен Deckhouse;

##### Объекты кластера:
* Все объекты `NodeGroup`;
* Все объекты `NodeGroupConfiguration`;
* Все объекты `Node`;
* Все объекты `Machine`;
* Все объекты `Instance`;
* Все объекты `StaticInstance`;
* Все объекты `MachineDeployment`;
* Все объекты `ClusterAuthorizationRule`;
* Все объекты `AuthorizationRule`;
* Все объекты `ModuleConfig`;
* `Events` из всех пространств имен;

##### Модули и их состояния:
* Список включенных модулей;
* Список объектов `ModuleSource` в кластере;
* Список объектов `ModulePullOverride` в кластере;
* Список модулей в режиме `maintenance`;

##### Логи и манифесты контроллеров:
* Логи machine controller manager;
* Логи cloud controller manager;
* Логи csi controller;
* Логи cluster autoscaler;
* Логи Vertical Pod Autoscaler admission controller;
* Логи Vertical Pod Autoscaler recommender;
* Логи Vertical Pod Autoscaler updater;
* YAML-файлы capi controller manager;
* YAML-файлы caps controller manager;
* YAML-файлы machine controller manager;

##### Мониторинг и алерты:
* Логи Prometheus;
* Все горящие уведомления в Prometheus;
* Список всех не поднятых подов, кроме тех, что в состояниях Completed и Evicted;

##### Сеть:
* Все объекты из Namespace `d8-istio`;
* Все Custom Resources istio;
* Envoy config istio;
* Логи istio;
* Логи istio ingressgateway;
* Логи istio users;
* Состояние связи Cilium - `cilium health status`;

## Как отлаживать проблемы в подах с помощью ephemeral containers?

Выполните следующую команду:

```shell
kubectl -n <namespace_name> debug -it <pod_name> --image=ubuntu <container_name>
```

Подробнее можно почитать в [официальной документации](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container).

## Как отлаживать проблемы на узлах с помощью ephemeral containers?

Выполните следующую команду:

```shell
kubectl debug node/mynode -it --image=ubuntu
```

Подробнее можно почитать в [официальной документации](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#node-shell-session).
