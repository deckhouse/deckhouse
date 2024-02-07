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
      summary: "Multiple default StorageClasses found in the cluster"
      description: |-
        More than one StorageClass in the cluster annotated as a default.
        Probably manually deployed StorageClass exists, that overlaps with cloud-provider module default Storage configuration.

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
      summary: "Manually deployed StorageClass `{{`{{ $labels.name }}`}}` found in the cluster"
      description: |-
        StorageClass having a cloud-provider provisioner shouldn't be deployed manually.
        They are managed by the cloud-provider module, you only need to change the module configuration to fit your needs.

        {{ include "cloud-provider-storage-documentation-url" . }}
