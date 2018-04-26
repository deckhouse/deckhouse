Модуль heapster
=======

Модуль устанавливает [heapster](https://github.com/kubernetes/heapster).

В скором времени вместо Heapster будет использоваться [Metrics API](https://github.com/kubernetes/metrics), но пока что Heapster необходим для работы следующих компонентов:
* Horizontal Pod Autoscaler (с версии 1.8 его можно полностью переключить на Metrics API, но для этого нужно kube-controller-manager запускать с флагом `--horizontal-pod-autoscaler-use-rest-clients`)
* kubectl top
* Рисовалка графиков в kubernetes dashboard (см. [подробнее тут](https://github.com/kubernetes/dashboard/issues/1310))

Heapster работает в standalone режиме (не использует никакой бекенд для хранения).

Конфигурация
------------

### Что нужно настраивать?

Обязательных настроек нет.

### Параметры

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/system","operator":"Exists"}]` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига

```yaml
heapster: |
  nodeSelector:
    node-role/other: ""
  tolerations:
  - key: node-role/other
    operator: Exists
```
