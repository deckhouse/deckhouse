# Vertical Pod Autoscaler

## Общее описание

Значения `minAllowed` и `maxAllowed` выбираются исходя из реального потребления CPU и memory контейнерами.

В случае, если модуль `vertical-pod-autoscaler` выключен, значения `minAllowed` используются для проставления реквестов контейнеров.

Лимиты для контейнеров не проставляются.

## Правила оформления

При написании нового модуля необходимо соблюдать следующие правила:

* Для любого deployment'а, statefulset'а или daemonset'а должен существовать соответствующий ресурс VPA, в котором должны быть описаны ресурсы по всем контейнерам, используемым в контроллере.  
* Описание VPA-ресурса должно находиться в отдельном файле `vpa.yaml`, который находится в папке с шаблонами модуля.
* `minAllowed`-ресурсы контейнера описываются при помощи helm-функции, которая находится в начале файла `vpa.yaml`.
* Для `maxAllowed`-ресурсов helm-функция необязательна.

> Внимание! Имя для helm-функций с `minAllowed`-ресурсами должно быть уникальным в пределах модуля.

Для контейнера `kube_rbac_proxy` используется функция `helm_lib_vpa_kube_rbac_proxy_resources`, которая проставляет и `minAllowed` и `maxAllowed` ресурсы.

Пример:

```yaml
{{- define "speaker_resources" }}
cpu: 10m
memory: 30Mi
{{- end }}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: speaker
  namespace: d8-{{ .Chart.Name }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: DaemonSet
    name: speaker
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: speaker
      minAllowed:
        {{- include "speaker_resources" . | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 60Mi
    {{- include "helm_lib_vpa_kube_rbac_proxy_resources" . | nindent 4 }}
```

Helm-функции, описанные в файле `vpa.yaml` используются так же для установки ресурсов контейнеров в случае, если модуль `vertical-pod-autoscaler` отключен.

Для проставления ресурсов для `kube-rbac-proxy` используется специальная helm-функция `helm_lib_container_kube_rbac_proxy_resources`.

Пример:

```yaml
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: speaker
  namespace: d8-{{ .Chart.Name }}
spec:
  selector:
    matchLabels:
      app: speaker
  template:
    metadata:
      labels:
        app: speaker
    spec: 
    containers:
      - name: speaker
        resources:
          requests:
          {{- if not ( .Values.global.enabledModules | has "vertical-pod-autoscaler") }}
            {{- include "speaker_resources" . | nindent 14 }}
          {{- end }}
      - name: kube-rbac-proxy
        resources:
          requests:
          {{- if not ( .Values.global.enabledModules | has "vertical-pod-autoscaler") }}
            {{- include "helm_lib_container_kube_rbac_proxy_resources" . | nindent 12 }}
          {{- end }}
```

## Специальные лейблы для VPA-ресурсов

Если Pod'ы присутствуют только на мастер-узлах, для VPA-ресурса добавляется label `workload-resource-policy.deckhouse.io: master`.

Если Pod'ы присутствуют на каждом узле, для VPA-ресурса добавляется label `workload-resource-policy.deckhouse.io: every-node`.

## TODO

* В настоящий момент для проставления ресурсов контейнеров используются значения из `minAllowed`. В этом случае возможен оверпровижининг на узле. Возможно правильнее было бы использовать значения `maxAllowed`.
* Значения `minAllowed` и `maxAllowed` проставляются вручную, возможно, определять нужно что-то одно, а второе вычислять. Например, определять `minAllowed` а `maxAllowed` считать как `minAllowed` X 2.
* Возможно стоит придумать другой механизм задания значений `minAllowed`, например, отдельный файл, в котором в YAML-структуре будут собраны данные по ресурсам всех контейнеров всех модулей.
* Issue #2084.
