# Vertical Pod Autoscaler

## General description

The `minAllowed` and `maxAllowed` values are chosen based on the actual CPU and memory consumption of the containers.

If the `vertical-pod-autoscaler` module is disabled, the `minAllowed` values are used to set the containers requests.

Limits for containers are not set.

## Design rules

When writing a new module, the following rules must be observed:

* For any deployment, statefulset or daemonset, there must be a corresponding VPA resource that describes the resources for all containers used in the controller.
* The description of the VPA resource should be in a separate file `vpa.yaml`, which is located in the folder with the module's templates.
* The `minAllowed` container resources are described using the helm function at the beginning of the `vpa.yaml` file.
* For `maxAllowed` resources the helm function is optional.

> **Note!** The name for helm-functions with `minAllowed`-resources must be unique within the module.

For the `kube_rbac_proxy` container, the function `helm_lib_vpa_kube_rbac_proxy_resources` is used to set both `minAllowed` and `maxAllowed` resources.

Example:

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

The helm functions described in the `vpa.yaml` file are also used to set container resources requests in case the `vertical-pod-autoscaler` module is disabled.

A special helm function `helm_lib_container_kube_rbac_proxy_resources` is used to set resources requests for `kube-rbac-proxy`.

Example:

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

## Special labels for VPA resources

If Pods should be present on master nodes, add label `workload-resource-policy.deckhouse.io: master` to the corresponding VPA resource.

If Pods should be present on every node, add label `workload-resource-policy.deckhouse.io: every-node` to the corresponding VPA resource.

## TODO

* At the moment container resources is set with values from `minAllowed`. It leads to possible node overprovision. Perhaps it would be more correct to use `maxAllowed` values.
* Values for `minAllowed` and `maxAllowed` set manually, perhaps that we need to determine one thing and calculate the other. For example, determine `minAllowed` and set `maxAllowed` as `minAllowed` X 2.
* Perhaps we should think of another mechanism for setting values `minAllowed`, for example, a separate file in which the yaml-structure will be collected data on the resources of all containers of all modules.
* [Issue #2084](https://github.com/deckhouse/deckhouse/issues/2084).

