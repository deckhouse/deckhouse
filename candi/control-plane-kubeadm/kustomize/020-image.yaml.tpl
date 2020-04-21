{{- if hasKey . "images" }}
  {{- range $component := (list "kube-apiserver" "kube-controller-manager" "kube-scheduler" "etcd") }}
    {{- if hasKey $.images ($component | camelcase) }}
---
apiVersion: v1
kind: Pod
metadata:
  name: {{ $component }}
  namespace: kube-system
spec:
  containers:
  - name: {{ $component }}
    image: {{ pluck ($component | camelcase) $.images }}
    {{- end }}
  {{- end }}
{{- end }}
