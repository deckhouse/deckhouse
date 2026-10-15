{{- define "schedulerConfig" }}
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
# kube-scheduler ignores the --profiling flag from the pod manifest as soon as --config is
# passed, and the manifest passes --config for every runType except ClusterBootstrap. The
# default here is "true", so profiling has to be turned off explicitly. Contention
# profiling is spelled out because upstream enables it by default when profiling is on,
# and it costs runtime.SetBlockProfileRate(1).
enableProfiling: false
enableContentionProfiling: false
clientConnection:
  kubeconfig: /etc/kubernetes/scheduler.conf
profiles:
- schedulerName: high-node-utilization
  plugins:
    score:
      disabled:
      - name: NodeResourcesBalancedAllocation
      - name: ImageLocality
      enabled:
      - name: NodeResourcesFit
        weight: 10
  pluginConfig:
  - name: NodeResourcesFit
    args:
      scoringStrategy:
        type: MostAllocated
        resources:
        - name: cpu
          weight: 1
        - name: memory
          weight: 1
  - name: PodTopologySpread
    args:
      defaultingType: List
      defaultConstraints: []
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
  httpTimeout: {{ $extender.timeout }}
  nodeCacheCapable: true
  ignorable: {{ $extender.ignorable }}
    {{- end }}
  {{- end }}
{{- end }}
