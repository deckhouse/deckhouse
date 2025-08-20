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

<table>
    <tr>
        <th>Категория</th>
        <th>Собираемые данные</th>
    </tr>
    <tr>
        <td>Deckhouse</td>
        <td>
            <ul>
                <li>Состояние очереди Deckhouse</li>
                <li>Deckhouse values. За исключением значений <code>kubeRBACProxyCA</code> и <code>registry.dockercfg</code></li>
                <li>Данные о текущей версии пода deckhouse</li>
                <li>Все объекты <code>DeckhouseRelease</code></li>
                <li>Логи подов Deckhouse</li>
                <li>Манифесты controller'ов и подов из всех пространств имен Deckhouse</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Объекты кластера</td>
        <td>
            <ul>
                <li>Все объекты <code>NodeGroup</code></li>
                <li>Все объекты <code>NodeGroupConfiguration</code></li>
                <li>Все объекты <code>Node</code></li>
                <li>Все объекты <code>Machine</code></li>
                <li>Все объекты <code>Instance</code></li>
                <li>Все объекты <code>StaticInstance</code></li>
                <li>Все объекты <code>MachineDeployment</code></li>
                <li>Все объекты <code>ClusterAuthorizationRule</code></li>
                <li>Все объекты <code>AuthorizationRule</code></li>
                <li>Все объекты <code>ModuleConfig</code></li>
                <li><code>Events</code> из всех пространств имен</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Модули и их состояния</td>
        <td>
            <ul>
                <li>Список включенных модулей</li>
                <li>Список объектов <code>ModuleSource</code> в кластере</li>
                <li>Список объектов <code>ModulePullOverride</code> в кластере</li>
                <li>Список модулей в режиме <code>maintenance</code></li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Логи и манифесты контроллеров</td>
        <td>
            <ul>
                <li>Логи machine controller manager</li>
                <li>Логи cloud controller manager</li>
                <li>Логи csi controller</li>
                <li>Логи cluster autoscaler</li>
                <li>Логи Vertical Pod Autoscaler admission controller</li>
                <li>Логи Vertical Pod Autoscaler recommender</li>
                <li>Логи Vertical Pod Autoscaler updater</li>
                <li>YAML-файлы capi controller manager</li>
                <li>YAML-файлы caps controller manager</li>
                <li>YAML-файлы machine controller manager</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Мониторинг и алерты</td>
        <td>
            <ul>
                <li>Логи Prometheus</li>
                <li>Все горящие уведомления в Prometheus</li>
                <li>Список всех не поднятых подов, кроме тех, что в состояниях Completed и Evicted</li>
            </ul>
        </td>
    </tr>
    <tr>
        <td>Сеть</td>
        <td>
            <ul>
                <li>Все объекты из Namespace <code>d8-istio</code></li>
                <li>Все Custom Resources istio</li>
                <li>Envoy config istio</li>
                <li>Логи istio</li>
                <li>Логи istio ingressgateway</li>
                <li>Логи istio users</li>
                <li>Состояние связи Cilium - <code>cilium health status</code></li>
            </ul>
        </td>
    </tr>
</table>

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
