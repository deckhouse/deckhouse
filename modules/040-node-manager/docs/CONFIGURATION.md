---
title: "Управление узлами: конфигурация"
---

Управление узлами осуществляется с помощью модуля `node-manager`, который по умолчанию — **включен**.

## Параметры

* `instancePrefix` — префикс, который следует использовать при создании instances в cloud provider.
  * Опциональный параметр.
  * Значение по умолчанию может вычисляться из custom resource `ClusterConfiguration`, если кластер был установлен инсталлятором Deckhouse.

### Примеры

```yaml
nodeManager: |
  instancePrefix: kube
```

### Правила выделения нод под специфические нагрузки

> Во всех случаях использование `dedicated.deckhouse.io` в ключе или его части не рекомендуется, данный ключ зарезервирован для использования внутри **Deckhouse**.

Для решений данной задачи существуют два механизма:
- Установка меток в `NodeGroup` `spec.nodeTemplate.labels`, для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает какие именно ноды будут выбраны планировщиком для запуска целевого приложения
- Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints`, с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих нодах.

Подробности [в статье на Habr](https://habr.com/ru/company/flant/blog/432748/).
