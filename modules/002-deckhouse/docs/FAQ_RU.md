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
   d8 system collect-debug-info > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Отправьте получившийся архив [команде Deckhouse](https://github.com/deckhouse/deckhouse/issues/new/choose) для дальнейшего расследования.

Данные, которые будут собраны:

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
          <li>Состояние очереди Deckhouse</li>
          <li>Deckhouse values (за исключением значений <code>kubeRBACProxyCA</code> и <code>registry.dockercfg</code>)</li>
          <li>Данные о текущей версии пода <code>deckhouse</code></li>
          <li>Все объекты DeckhouseRelease</li>
          <li>Логи подов Deckhouse</li>
          <li>Манифесты контроллеров и подов из всех пространств имен Deckhouse</li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Объекты кластера</strong></td>
      <td>
        Все объекты следующих ресурсов:
        <ul>
          <li>NodeGroup</li>
          <li>NodeGroupConfiguration</li>
          <li>Node</li>
          <li>Machine</li>
          <li>Instance</li>
          <li>StaticInstance</li>
          <li>MachineDeployment</li>
          <li>ClusterAuthorizationRule</li>
          <li>AuthorizationRule</li>
          <li>ModuleConfig</li>
        </ul>
        А также Events из всех пространств имен
      </td>
    </tr>
    <tr>
      <td><strong>Модули и их состояния</strong></td>
      <td>
        <ul>
          <li>Список включенных модулей</li>
          <li>Список объектов ModuleSource в кластере</li>
          <li>Список объектов ModulePullOverride в кластере</li>
          <li>Список модулей в режиме <code>maintenance</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Логи и манифесты контроллеров</strong></td>
      <td>
        Логи следующих компонентов:
        <ul>
          <li><code>machine-controller-manager</code></li>
          <li><code>cloud-controller-manager</code></li>
          <li><code>csi-controller</code></li>
          <li><code>cluster-autoscaler</code></li>
          <li>Vertical Pod Autoscaler admission controller</li>
          <li>Vertical Pod Autoscaler recommender</li>
          <li>Vertical Pod Autoscaler updater</li>
        </ul>
        YAML-файлы следующих контроллеров:
        <ul>
          <li><code>capi-controller-manager</code></li>
          <li><code>caps-controller-manager</code></li>
          <li><code>machine-controller-manager</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Мониторинг и алерты</strong></td>
      <td>
        <ul>
          <li>Логи Prometheus</li>
          <li>Все горящие уведомления в Prometheus</li>
          <li>Список всех подов, которые не находятся в состоянии <code>Running</code>, кроме подов в состояниях <code>Completed</code> и <code>Evicted</code></li>
        </ul>
      </td>
    </tr>
    <tr>
      <td><strong>Сеть</strong></td>
      <td>
        <ul>
          <li>Все объекты из пространства имен <code>d8-istio</code></li>
          <li>Все кастомные ресурсы <code>istio</code></li>
          <li>Конфигурация Envoy для <code>istio</code></li>
          <li>Логи <code>istio</code></li>
          <li>Логи <code>istio</code> ingress gateway</li>
          <li>Логи <code>istio</code> users</li>
          <li>Состояние соединения Cilium (<code>cilium health status</code>)</li>
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
