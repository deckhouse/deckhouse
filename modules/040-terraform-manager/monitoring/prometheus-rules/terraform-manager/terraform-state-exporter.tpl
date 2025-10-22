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
      summary: Prometheus can't scrape terraform-state-exporter.
      description: |
        Prometheus is unable to scrape metrics from the `terraform-state-exporter`.

        To investigate the details:

        - Check the Pod status:

          ```shell
          kubectl -n d8-system get pod -l app=terraform-state-exporter
          ```

        - Check the container logs:

          ```shell
          kubectl -n d8-system logs -l app=terraform-state-exporter -c exporter
          ```

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
      summary: Terraform-state-exporter target is missing in Prometheus.
      description: |
        Prometheus cannot find the `terraform-state-exporter` target.

        To investigate the details:

        - Check the Pod status:

          ```shell
          kubectl -n d8-system get pod -l app=terraform-state-exporter
          ```

        - Check the container logs:

          ```shell
          kubectl -n d8-system logs -l app=terraform-state-exporter -c exporter
          ```

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
      summary: Terraform-state-exporter Pod is not Ready.
      description: |
        The `terraform-state-exporter` cannot check the difference between the actual Kubernetes cluster state and the Terraform state.

        To resolve the issue, check the following:

        1. Deployment description:

           ```shell
           kubectl -n d8-system describe deployment terraform-state-exporter
           ```

        2. Pod status:

           ```shell
           kubectl -n d8-system describe pod -l app=terraform-state-exporter
           ```

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
      summary: Terraform-state-exporter Pod is not Running.
      description: |
        The `terraform-state-exporter` cannot check the difference between the actual Kubernetes cluster state and the Terraform state.

        To resolve the issue, check the following:

        1. Deployment description:

           ```shell
           kubectl -n d8-system describe deployment terraform-state-exporter
           ```

        2. Pod status:

           ```shell
           kubectl -n d8-system describe pod -l app=terraform-state-exporter
           ```

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
      summary: Terraform-state-exporter has encountered errors.
      description: |
        Errors occurred during the operation of the `terraform-state-exporter`.

        To get more details, check the Pod logs:

        ```shell
        kubectl -n d8-system logs -l app=terraform-state-exporter -c exporter
        ```

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
      summary: Terraform-state-exporter cluster state change detected.
      description: |
        The current Kubernetes cluster state is `{{`{{ $labels.status }}`}}` compared to the Terraform state.

        It is important to reconcile the states.

        Troubleshooting steps:

        1. View the differences:

           ```shell
           dhctl terraform check
           ```

        2. Apply the necessary changes to bring the cluster in sync:

           ```shell
           dhctl converge
           ```

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
      summary: Terraform-state-exporter node state change detected.
      description: |
        The current state of node `{{"{{ $labels.node_group }}/{{ $labels.name }}"}}` is `{{`{{ $labels.status }}`}}` compared to the Terraform state.

        It is important to reconcile the states.

        Troubleshooting steps:

        1. View the differences:

           ```shell
           dhctl terraform check
           ```

        2. Apply the necessary changes to bring the cluster in sync:

           ```shell
           dhctl converge
           ```

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
      summary: Terraform-state-exporter cluster state error.
      description: |
        The `terraform-state-exporter` can't check difference between the Kubernetes cluster state and the Terraform state.

        That was likely caused by `terraform-state-exporter` failing to run Terraform with the current state and configuration.

        Troubleshooting steps:

        1. View the differences:

           ```shell
           dhctl terraform check
           ```

        2. Apply the necessary changes to bring the cluster in sync:

           ```shell
           dhctl converge
           ```

{{- if (.Values.global.enabledModules | has "cloud-provider-aws") }}
{{ if .Values.global.modules.publicDomainTemplate }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- else }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user](https://deckhouse.io/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- end }}
        1. `ec2:DescribeInstanceTypes`,
        2. `ec2:DescribeSecurityGroupRules`.
{{- end }}

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
      summary: Terraform-state-exporter node state error.
      description: |
        The `terraform-state-exporter` can't check the difference between the node `{{"{{ $labels.node_group }}/{{ $labels.name }}"}}` state and the Terraform state.

        Probably, it occurred because `terraform-manager` had failed to run Terraform with the current state and configuration.

        Troubleshooting steps:

        1. View the differences:

           ```shell
           dhctl terraform check
           ```

        2. Apply the necessary changes to bring the cluster in sync:

           ```shell
           dhctl converge
           ```

{{- if (.Values.global.enabledModules | has "cloud-provider-aws") }}
{{ if .Values.global.modules.publicDomainTemplate }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- else }}
        Also, it can occur because of missing permissions for the following actions for the [Deckhouse IAM user](https://deckhouse.io/modules/cloud-provider-aws/environment.html#json-policy) in AWS (the new requirements in Deckhouse 1.45):
{{- end }}
        1. `ec2:DescribeInstanceTypes`,
        2. `ec2:DescribeSecurityGroupRules`.
{{- end }}

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
      summary: Terraform-state-exporter node template change detected.
      description: |
        The `terraform-state-exporter` has detected a mismatch between the node template in the cluster provider configuration and the one specified in the NodeGroup {{`{{ $labels.name }}`}}`.

        Node template is `{{`{{ $labels.status }}`}}`.

        Troubleshooting steps:

        1. View the differences:

           ```shell
           dhctl terraform check
           ```

        2. Adjust NodeGroup settings to fix the issue or bring the cluster in sync via the following command:

           ```shell
           dhctl converge
           ```
