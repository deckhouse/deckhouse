{{- define "schedulerConfig" }}
apiVersion: kubescheduler.config.k8s.io/v1
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
  {{- if $extender.filterVerb }}
  filterVerb: {{ $extender.filterVerb}}
  {{- end }}
  {{- if $extender.prioritizeVerb }}
  prioritizeVerb: {{ $extender.prioritizeVerb}}
  {{- end }}
  {{- if $extender.preemptVerb }}
  preemptVerb: {{ $extender.preemptVerb}}
  {{- end }}
  weight: {{ $extender.weight }}
  enableHTTPS: true
  tlsConfig:
    caData: {{ $extender.caData }}
  httpTimeout: {{ $extender.timeout }}s
  nodeCacheCapable: true
  ignorable: {{ $extender.ignorable }}
    {{- end }}
  {{- end }}
{{- end }}
