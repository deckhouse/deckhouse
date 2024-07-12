---
title: "Модуль deckhouse: FAQ"
---

## Как запустить kube-bench в кластере?

Вначале необходимо зайти внутрь пода Deckhouse:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- bash
```

Далее необходимо выбрать, на каком узле запустить kube-bench.

* Запуск на случайном узле:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
  ```

* Запуск на конкретном узле, например на control-plane:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl apply -f - --dry-run=client -o json | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
  ```

Далее можно проверить результат выполнения:

```shell
kubectl logs job.batch/kube-bench
```

## Как собрать информацию для отладки?

Мы всегда рады помочь пользователям с расследованием сложных проблем. Пожалуйста, выполните следующие шаги, чтобы мы смогли вам помочь:

1. Выполните следующую команду, чтобы собрать необходимые данные:

   ```sh
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Отправьте получившийся архив [команде Deckhouse](https://github.com/deckhouse/deckhouse/issues/new/choose) для дальнейшего расследования.

Данные, которые будут собраны:
* состояние очереди Deckhouse;
* Deckhouse values;
* список включенных модулей;
* `events` из всех пространств имен;
* манифесты controller'ов и подов из всех пространств имен Deckhouse;
* все объекты `nodegroups`;
* все объекты `nodes`;
* все объекты `machines`;
* данные о текущей версии пода deckhouse;
* все объекты `deckhousereleases`;
* логи Deckhouse;
* логи machine controller manager;
* логи cloud controller manager;
* логи cluster autoscaler;
* логи Vertical Pod Autoscaler admission controller;
* логи Vertical Pod Autoscaler recommender;
* логи Vertical Pod Autoscaler updater;
* логи Prometheus;
* метрики terraform-state-exporter;
* все горящие уведомления в Prometheus.

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
