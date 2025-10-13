---
title: "Модуль deckhouse: FAQ"
---

## Как запустить kube-bench в кластере?

1. Зайдите внутрь пода Deckhouse:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- bash
   ```

1. Выберите, на каком узле запустить kube-bench.

   * Запуск на случайном узле:

     ```shell
     curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | d8 k create -f -
     ```

   * Запуск на конкретном узле, например на control-plane:

     ```shell
     curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | d8 k apply -f - --dry-run=client -o json | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | d8 k create -f -
     ```

1. Проверьте результат выполнения:

   ```shell
   d8 k logs job.batch/kube-bench
   ```

{% alert level="warning" %}
В Deckhouse установлен срок хранения логов — 7 дней. Однако, в соответствии с требованиями безопасности указанными в kube-bench, логи должны храниться не менее 30 дней. Используйте отдельное хранилище для логов, если вам необходимо хранить логи более 7 дней.
{% endalert %}

## Как собрать информацию для отладки?

Мы всегда рады помочь пользователям с расследованием сложных проблем. Пожалуйста, выполните следующие шаги, чтобы мы смогли вам помочь:

1. Выполните следующую команду, чтобы собрать необходимые данные:

   ```sh
   d8 p collect-debug-info > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

{% alert level="info" %}
Флаг `--exclude` позволяет исключить файлы, данные по которым не будут включены в архив.

```sh
d8 p collect-debug-info --exclude=queue global-values > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
```

Флаг `--list-exclude` отображает список файлов, которые можно исключить из выборки.
{% endalert %}

2. Отправьте получившийся архив [команде Deckhouse](https://github.com/deckhouse/deckhouse/issues/new/choose) для дальнейшего расследования.

Данные, которые будут собраны:

<table>
  <thead>
    <tr>
      <th>Категория</th>
      <th>Собираемые данные</th>
      <th>Файл в архиве</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td rowspan="6"><strong>Deckhouse</strong></td>
      <td>Состояние очереди Deckhouse</td>
      <td><code>queue</code></td>
    </tr>
    <tr>
      <td>Deckhouse values (за исключением значений <code>kubeRBACProxyCA</code> и <code>registry.dockercfg</code>)</td>
      <td><code>global-values</code></td>
    </tr>
    <tr>
      <td>Данные о текущей версии пода <code>deckhouse</code></td>
      <td><code>deckhouse-version</code></td>
    </tr>
    <tr>
      <td>Все объекты DeckhouseRelease</td>
      <td><code>deckhouse-releases</code></td>
    </tr>
    <tr>
      <td>Логи подов Deckhouse</td>
      <td><code>deckhouse-logs</code></td>
    </tr>
    <tr>
      <td>Манифесты контроллеров и подов из всех пространств имен Deckhouse</td>
      <td><code>d8-all</code></td>
    </tr>
    <tr>
      <td rowspan="11"><strong>Объекты кластера</strong></td>
      <td>NodeGroup</td>
      <td><code>node-groups</code></td>
    </tr>
    <tr>
      <td>NodeGroupConfiguration</td>
      <td><code>node-group-configuration</code></td>
    </tr>
    <tr>
      <td>Node</td>
      <td><code>nodes</code></td>
    </tr>
    <tr>
      <td>Machine</td>
      <td><code>machines</code></td>
    </tr>
    <tr>
      <td>Instance</td>
      <td><code>instances</code></td>
    </tr>
    <tr>
      <td>StaticInstance</td>
      <td><code>staticinstances</code></td>
    </tr>
    <tr>
      <td>MachineDeployment</td>
      <td><code>cloud-machine-deployment</code>, <code>static-machine-deployment</code></td>
    </tr>
    <tr>
      <td>ClusterAuthorizationRule</td>
      <td><code>cluster-authorization-rules</code></td>
    </tr>
    <tr>
      <td>AuthorizationRule</td>
      <td><code>authorization-rules</code></td>
    </tr>
    <tr>
      <td>ModuleConfig</td>
      <td><code>module-configs</code></td>
    </tr>
    <tr>
      <td>Events из всех пространств имен</td>
      <td><code>events</code></td>
    </tr>
    <tr>
      <td rowspan="4"><strong>Модули и их состояния</strong></td>
      <td>Список включенных модулей</td>
      <td><code>deckhouse-enabled-modules</code></td>
    </tr>
    <tr>
      <td>Список объектов ModuleSource в кластере</td>
      <td><code>deckhouse-module-sources</code></td>
    </tr>
    <tr>
      <td>Список объектов ModulePullOverride в кластере</td>
      <td><code>deckhouse-module-pull-overrides</code></td>
    </tr>
    <tr>
      <td>Список модулей в режиме <code>maintenance</code></td>
      <td><code>deckhouse-maintenance-modules</code></td>
    </tr>
    <tr>
      <td rowspan="10"><strong>Логи и манифесты контроллеров</strong></td>
      <td>Логи <code>machine-controller-manager</code></td>
      <td><code>mcm-logs</code></td>
    </tr>
    <tr>
      <td>Логи <code>cloud-controller-manager</code></td>
      <td><code>ccm-logs</code></td>
    </tr>
    <tr>
      <td>Логи <code>csi-controller</code></td>
      <td><code>csi-controller-logs</code></td>
    </tr>
    <tr>
      <td>Логи <code>cluster-autoscaler</code></td>
      <td><code>cluster-autoscaler-logs</code></td>
    </tr>
    <tr>
      <td>Логи Vertical Pod Autoscaler admission controller</td>
      <td><code>vpa-admission-controller-logs</code></td>
    </tr>
    <tr>
      <td>Логи Vertical Pod Autoscaler recommender</td>
      <td><code>vpa-recommender-logs</code></td>
    </tr>
    <tr>
      <td>Логи Vertical Pod Autoscaler updater</td>
      <td><code>vpa-updater-logs</code></td>
    </tr>
    <tr>
      <td>YAML <code>capi-controller-manager</code></td>
      <td><code>capi-controller-manager</code></td>
    </tr>
    <tr>
      <td>YAML <code>caps-controller-manager</code></td>
      <td><code>caps-controller-manager</code></td>
    </tr>
    <tr>
      <td>YAML <code>machine-controller-manager</code></td>
      <td><code>machine-controller-manager</code></td>
    </tr>
    <tr>
      <td rowspan="4"><strong>Мониторинг и алерты</strong></td>
      <td>Логи Prometheus</td>
      <td><code>prometheus-logs</code></td>
    </tr>
    <tr>
      <td>Все горящие уведомления в Prometheus</td>
      <td><code>alerts</code></td>
    </tr>
    <tr>
      <td>Список всех подов, которые не находятся в состоянии <code>Running</code>, кроме подов в состояниях <code>Completed</code> и <code>Evicted</code></td>
      <td><code>bad-pods</code></td>
    </tr>
    <tr>
      <td>Список Audit Policy</td>
      <td><code>audit-policy</code></td>
    </tr>
    <tr>
      <td rowspan="7"><strong>Сеть</strong></td>
      <td>Все объекты из пространства имен <code>d8-istio</code></td>
      <td><code>d8-istio-resources</code></td>
    </tr>
    <tr>
      <td>Все кастомные ресурсы <code>istio</code></td>
      <td><code>d8-istio-custom-resources</code></td>
    </tr>
    <tr>
      <td>Конфигурация Envoy для <code>istio</code></td>
      <td><code>d8-istio-envoy-config</code></td>
    </tr>
    <tr>
      <td>Логи <code>istio</code></td>
      <td><code>d8-istio-system-logs</code></td>
    </tr>
    <tr>
      <td>Логи <code>istio</code> ingress gateway</td>
      <td><code>d8-istio-ingress-logs</code></td>
    </tr>
    <tr>
      <td>Логи <code>istio</code> users</td>
      <td><code>d8-istio-users-logs</code></td>
    </tr>
    <tr>
      <td>Состояние соединения Cilium (<code>cilium health status</code>)</td>
      <td><code>cilium-health-status</code></td>
    </tr>
  </tbody>
</table>

## Как отлаживать проблемы в подах с помощью ephemeral containers?

Выполните следующую команду:

```shell
d8 k -n <namespace_name> debug -it <pod_name> --image=ubuntu <container_name>
```

Подробнее можно почитать в [официальной документации](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container).

## Как отлаживать проблемы на узлах с помощью ephemeral containers?

Выполните следующую команду:

```shell
d8 k debug node/mynode -it --image=ubuntu
```

Подробнее можно почитать в [официальной документации](https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#node-shell-session).
