{{- define "schedulerConfig" }}
{{- if semverCompare ">= 1.23" .clusterConfiguration.kubernetesVersion }}
apiVersion: kubescheduler.config.k8s.io/v1beta3
{{- else if semverCompare "= 1.22" .clusterConfiguration.kubernetesVersion }}
apiVersion: kubescheduler.config.k8s.io/v1beta2
{{- else }}
apiVersion: kubescheduler.config.k8s.io/v1beta1
{{- end }}
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
