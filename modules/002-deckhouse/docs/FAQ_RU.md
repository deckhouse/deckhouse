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

Данные, которые будут собраны (через "/" указаны название файлов, который будет содержать соответствующие данные):

<table>
  <thead>
    <tr>
      <th>Категория</th>
      <th>Собираемые данные</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>Deckhouse</strong></td>
      <td>
        <ul>
          <li>Состояние очереди Deckhouse / <code>queue</code></li>
          <li>Deckhouse values (за исключением значений <code>kubeRBACProxyCA</code> и <code>registry.dockercfg</code>) / <code>global-values</code></li>
          <li>Данные о текущей версии пода <code>deckhouse</code> / <code>deckhouse-version</code></li>
          <li>Все объекты DeckhouseRelease / <code>deckhouse-releases</code></li>
          <li>Логи подов Deckhouse / <code>deckhouse-logs</code></li>
          <li>Манифесты контроллеров и подов из всех пространств имен Deckhouse / <code>d8-all</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Объекты кластера</strong></td>
      <td>
        Все объекты следующих ресурсов:
        <ul>
          <li>NodeGroup / <code>node-groups</code></li>
          <li>NodeGroupConfiguration / <code>node-group-configuration</code></li>
          <li>Node / <code>nodes</code></li>
          <li>Machine / <code>machines</code></li>
          <li>Instance / <code>instances</code></li>
          <li>StaticInstance / <code>staticinstances</code></li>
          <li>MachineDeployment / <code>cloud-machine-deployment</code>, <code>static-machine-deployment</code></li>
          <li>ClusterAuthorizationRule / <code>cluster-authorization-rules</code></li>
          <li>AuthorizationRule / <code>authorization-rules</code></li>
          <li>ModuleConfig / <code>module-configs</code></li>
        </ul>
        А также Events из всех пространств имен / <code>events</code>
      </td>
    </tr>
    <tr>
      <td><strong>Модули и их состояния</strong></td>
      <td>
        <ul>
          <li>Список включенных модулей / <code>deckhouse-enabled-modules</code></li>
          <li>Список объектов ModuleSource в кластере / <code>deckhouse-module-sources</code></li>
          <li>Список объектов ModulePullOverride в кластере / <code>deckhouse-module-pull-overrides</code></li>
          <li>Список модулей в режиме <code>maintenance</code> / <code>deckhouse-maintenance-modules</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Логи и манифесты контроллеров</strong></td>
      <td>
        Логи следующих компонентов:
        <ul>
          <li><code>machine-controller-manager</code> / <code>mcm-logs</code></li>
          <li><code>cloud-controller-manager</code> / <code>ccm-logs</code></li>
          <li><code>csi-controller</code> / <code>csi-controller-logs</code></li>
          <li><code>cluster-autoscaler</code> / <code>cluster-autoscaler-logs</code></li>
          <li>Vertical Pod Autoscaler admission controller / <code>vpa-admission-controller-logs</code></li>
          <li>Vertical Pod Autoscaler recommender / <code>vpa-recommender-logs</code></li>
          <li>Vertical Pod Autoscaler updater / <code>vpa-updater-logs</code></li>
        </ul>
        YAML-файлы следующих контроллеров:
        <ul>
          <li><code>capi-controller-manager</code> / <code>capi-controller-manager</code></li>
          <li><code>caps-controller-manager</code> / <code>caps-controller-manager</code></li>
          <li><code>machine-controller-manager</code> / <code>machine-controller-manager</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Мониторинг и алерты</strong></td>
      <td>
        <ul>
          <li>Логи Prometheus / <code>prometheus-logs</code></li>
          <li>Все горящие уведомления в Prometheus / <code>alerts</code></li>
          <li>Список всех подов, которые не находятся в состоянии <code>Running</code>, кроме подов в состояниях <code>Completed</code> и <code>Evicted</code> / <code>bad-pods</code></li>
          <li>Список Audit Policy / <code>audit-policy</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Сеть</strong></td>
      <td>
        <ul>
          <li>Все объекты из пространства имен <code>d8-istio</code> / <code>d8-istio-resources</code></li>
          <li>Все кастомные ресурсы <code>istio</code> / <code>d8-istio-custom-resources</code></li>
          <li>Конфигурация Envoy для <code>istio</code> / <code>d8-istio-envoy-config</code></li>
          <li>Логи <code>istio</code> / <code>d8-istio-system-logs</code></li>
          <li>Логи <code>istio</code> ingress gateway / <code>d8-istio-ingress-logs</code></li>
          <li>Логи <code>istio</code> users / <code>d8-istio-users-logs</code></li>
          <li>Состояние соединения Cilium (<code>cilium health status</code>) / <code>cilium-health-status</code></li>
        </ul>
      </td>
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
