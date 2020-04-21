{{- range $component := (list "kube-apiserver" "kube-controller-manager" "kube-scheduler") }}
---
apiVersion: v1
kind: Pod
metadata:
  name: {{ $component }}
  namespace: kube-system
  annotations:
    control-plane-manager.deckhouse.io/kubernetes-version: {{ $.clusterConfiguration.kubernetesVersion | quote }}
{{- end }}
