{{- define "schedulerConfig" }}
apiVersion: kubescheduler.config.k8s.io/v1beta3
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: /etc/kubernetes/scheduler.conf
profiles:
- pluginConfig:
  - name: PodTopologySpread
    args:
      {{- if semverCompare ">= 1.22" .clusterConfiguration.kubernetesVersion }}
      defaultingType: List
      {{- end }}
      defaultConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: ScheduleAnyway
{{- end }}
