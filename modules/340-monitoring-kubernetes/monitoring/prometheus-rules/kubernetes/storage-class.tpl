{{- define "cloud-provider-storage-documentation-url" }}
{{- if .Values.global.modules.publicDomainTemplate }}
  {{- if ( .Values.global.enabledModules | has "cloud-provider-aws") }}
        [AWS storage documentation is here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/030-cloud-provider-aws/configuration.html#storage).
  {{- else if ( .Values.global.enabledModules | has "cloud-provider-gcp") }}
        [GCP storage documentation is here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/030-cloud-provider-gcp/configuration.html#storage).
  {{- else if ( .Values.global.enabledModules | has "cloud-provider-openstack") }}
        [Openstack storage documentation is here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/030-cloud-provider-openstack/configuration.html#storage).
  {{- else if ( .Values.global.enabledModules | has "cloud-provider-vsphere") }}
        [vSphere storage documentation is here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/030-cloud-provider-vsphere/configuration.html#storage).
  {{- else if ( .Values.global.enabledModules | has "cloud-provider-yandex") }}
        [Yandex storage documentation is here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/modules/030-cloud-provider-yandex/configuration.html#storage).
  {{- else }}
        [Find storage configuration documentation for your cloud-provider here]({{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}/kubernetes.html).
  {{- end }}
{{- end }}
{{- end }}

- name: d8.monitoring-kubernetes.storage-class
  rules:
  - alert: StorageClassDefaultDuplicate
    expr: sum(storage_class_default_duplicate == 1) > 1
    labels:
      d8_component: monitoring-kubernetes
      d8_module: monitoring-kubernetes
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Multiple default StorageClasses found in the cluster.
      description: |-
        Deckhouse has detected that more than one StorageClass in the cluster is annotated as default.
        
        This may have been caused by a manually deployed StorageClass that is overlapping with the default storage configuration provided by the `cloud-provider` module.

        {{ include "cloud-provider-storage-documentation-url" . }}

  - alert: StorageClassCloudManual
    expr: max(storage_class_cloud_manual) by (name) == 1
    labels:
      d8_component: monitoring-kubernetes
      d8_module: monitoring-kubernetes
      severity_level: "6"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Manually deployed StorageClass `{{`{{ $labels.name }}`}}` found in the cluster.
      description: |-
        A StorageClass using a `cloud-provider` provisioner shouldn't be deployed manually.
        Such StorageClasses are managed by the `cloud-provider` module.
        
        Instead of the manual deployment, modify the `cloud-provider` module configuration as needed.

        {{ include "cloud-provider-storage-documentation-url" . }}
