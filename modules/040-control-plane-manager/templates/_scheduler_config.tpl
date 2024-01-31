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
- pluginConfig:
  - name: PodTopologySpread
    args:
      defaultingType: List
      defaultConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
{{- end }}
