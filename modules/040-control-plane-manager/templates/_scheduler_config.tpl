{{- define "schedulerConfig" }}
  {{- if semverCompare ">= 1.29" .clusterConfiguration.kubernetesVersion }}
apiVersion: kubescheduler.config.k8s.io/v1
  {{- else }}
apiVersion: kubescheduler.config.k8s.io/v1beta3
  {{- end }}
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: /etc/kubernetes/scheduler.conf
profiles:
- schedulerName: high-node-utilization
  pluginConfig:
  - args:
      scoringStrategy:
        resources:
        - name: cpu
          weight: 1
        - name: memory
          weight: 1
        type: MostAllocated
    name: NodeResourcesFit
- schedulerName: default-scheduler
  pluginConfig:
  - name: PodTopologySpread
    args:
      defaultingType: List
      defaultConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
  {{- if .scheduler.extenders }}
extenders:
    {{- range $extender := .scheduler.extenders }}
- urlPrefix: {{ $extender.urlPrefix }}
  filterVerb: filter
  prioritizeVerb: prioritize
  weight: {{ $extender.weight }}
  enableHTTPS: true
  httpTimeout: {{ $extender.timeout }}s
  nodeCacheCapable: true
  ignorable: {{ $extender.ignorable }}
  tlsConfig:
    caData: {{ $extender.caData }}
    {{- end }}
  {{- end }}
{{- end }}
