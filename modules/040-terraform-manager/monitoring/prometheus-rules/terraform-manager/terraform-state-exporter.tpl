- name: d8.terraform-manager.terraform-state-exporter.availability
  rules:

  - alert: D8TerraformStateExporterTargetDown
    expr: max by (job) (up{job="terraform-state-exporter"} == 0)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_ignore_labels: "job"
      description: >
        To get more details:

        Check pods state: `kubectl -n d8-system get pod -l app=terraform-state-exporter`
        or logs: `kubectl -n d8-system logs -l app=terraform-state-exporter -c exporter`
      summary: Prometheus can't scrape terraform-state-exporter

  - alert: D8TerraformStateExporterTargetAbsent
    expr: absent(up{job="terraform-state-exporter"}) == 1
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_ignore_labels: "job"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: >
        To get more details:

        Check pods state: `kubectl -n d8-system get pod -l app=terraform-state-exporter`
        or logs: `kubectl -n d8-system logs -l app=terraform-state-exporter -c exporter`
      summary: Prometheus has no `terraform-state-exporter` target

  - alert: D8TerraformStateExporterPodIsNotReady
    expr: |
      min by (pod) (
        kube_controller_pod{namespace="d8-system", controller_type="Deployment", controller_name="terraform-state-exporter"}
        * on (pod) group_right() kube_pod_status_ready{condition="true", namespace="d8-system"}
      ) != 1
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_labels_as_annotations: "pod"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Pod terraform-state-exporter is not Ready
      description: |
        Terraform-state-exporter doesn't check the difference between real Kubernetes cluster state and Terraform state.

        Pease, check:
        1. Deployment description: `kubectl -n d8-system describe deploy terraform-state-exporter`
        2. Pod status: `kubectl -n d8-system describe pod -l app=terraform-state-exporter`

  - alert: D8TerraformStateExporterPodIsNotRunning
    expr: absent(kube_pod_status_phase{namespace="d8-system",phase="Running",pod=~"terraform-state-exporter-.*"})
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      summary: Pod terraform-state-exporter is not Running
      description: |
        Terraform-state-exporter doesn't check the difference between real Kubernetes cluster state and Terraform state.

        Pease, check:
        1. Deployment description: `kubectl -n d8-system describe deploy terraform-state-exporter`
        2. Pod status: `kubectl -n d8-system describe pod -l app=terraform-state-exporter`

- name: d8.terraform-manager.terraform-state-exporter.checks
  rules:

  - alert: D8TerraformStateExporterHasErrors
    expr: |
      increase(candi_converge_exporter_errors{job="terraform-state-exporter"}[5m]) == 3
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Errors occurred while terraform-state-exporter working.

        Check pods logs to get more details: `kubectl -n d8-system logs -l app=terraform-state-exporter -c exporter`
      summary: Terraform-state-exporter has errors

  - alert: D8TerraformStateExporterClusterStateChanged
    expr: |
      max by(job, status) (candi_converge_cluster_status{status=~"changed|destructively_changed", job="terraform-state-exporter"} == 1)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Real Kubernetes cluster state is `{{`{{ $labels.status }}`}}` comparing to Terraform state.

        It's important to make them equal.
        First, run the `dhctl terraform check` command to check what will change.
        To converge state of Kubernetes cluster, use `dhctl converge` command.
      summary: Terraform-state-exporter cluster state changed

  - alert: D8TerraformStateExporterNodeStateChanged
    expr: |
      max by(node_group, name, status) (candi_converge_node_status{status=~"changed|destructively_changed|absent|abandoned", job="terraform-state-exporter"} == 1)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Real Node `{{"{{ $labels.node_group }}/{{ $labels.name }}"}}` state is `{{`{{ $labels.status }}`}}` comparing to Terraform state.

        It's important to make them equal.
        First, run the `dhctl terraform check` command to check what will change.
        To converge state of Kubernetes cluster, use `dhctl converge` command.
      summary: Terraform-state-exporter node state changed

  - alert: D8TerraformStateExporterClusterStateError
    expr: |
      max by(job) (candi_converge_cluster_status{status="error", job="terraform-state-exporter"} == 1)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Terraform-state-exporter can't check difference between Kubernetes cluster state and Terraform state.

        Probably, it occurred because Terraform-state-exporter had failed to run terraform with current state and config.
        First, run the `dhctl terraform check` command to check what will change.
        To converge state of Kubernetes cluster, use `dhctl converge` command.

{{- if (.Values.global.enabledModules | has "cloud-provider-aws") }}
{{ if .Values.global.modules.publicDomainTemplate }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/products/kubernetes-platform/documentation/v1/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- else }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- end }}
        1. `ec2:DescribeInstanceTypes`,
        2. `ec2:DescribeSecurityGroupRules`.
{{- end }}
      summary: Terraform-state-exporter cluster state error

  - alert: D8TerraformStateExporterNodeStateError
    expr: |
      max by(node_group, name) (candi_converge_node_status{status="error", job="terraform-state-exporter"} == 1)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Terraform-state-exporter can't check difference between Node `{{"{{ $labels.node_group }}/{{ $labels.name }}"}}` state and Terraform state.

        Probably, it occurred because Terraform-manager had failed to run terraform with current state and config.
        First, run the `dhctl terraform check` command to check what will change.
        To converge state of Kubernetes cluster, use `dhctl converge` command.

{{- if (.Values.global.enabledModules | has "cloud-provider-aws") }}
{{ if .Values.global.modules.publicDomainTemplate }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/products/kubernetes-platform/documentation/v1/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- else }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- end }}
        1. `ec2:DescribeInstanceTypes`,
        2. `ec2:DescribeSecurityGroupRules`.
{{- end }}
      summary: Terraform-state-exporter node state error

  - alert: D8TerraformStateExporterNodeTemplateChanged
    expr: |
      max by(job) (candi_converge_node_template_status{status!="ok", job="terraform-state-exporter"} == 1)
    for: 10m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_terraform_state_exporter_malfunctioning: "D8TerraformStateExporterMalfunctioning,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      description: |
        Terraform-state-exporter found difference between node template from cluster provider configuration and from NodeGroup `{{`{{ $labels.name }}`}}`.
        Node template is `{{`{{ $labels.status }}`}}`.

        First, run the `dhctl terraform check` command to check what will change.
        Use `dhctl converge` command or manually adjust NodeGroup settings to fix the issue.
      summary: Terraform-state-exporter node template changed


  - alert: D8TerraformVersionMismatch
    expr: |
      candi_converge_terraform_state_version unless on(version) candi_converge_terraform_current_version
    for: 5m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: terraform-manager
      d8_component: terraform-state-exporter
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: "Terraform version mismatch detected"
      description: |
        The Terraform version in the state does not match the current Terraform version.
        
        Please verify:
        - Current version: `kubectl -n d8-system exec -it deployments/terraform-state-exporter -c exporter -- terraform version`
        - State version: `kubectl -n d8-system get secret d8-cluster-terraform-state -o json | jq -r '.data["cluster-tf-state.json"] | @base64d | fromjson | .terraform_version'`
