{{- define "image-availability-alerts-by-mode" }}
{{- $controllerKind := . }}

- alert: {{ $controllerKind }}ImageAbsent
  expr: |
    max by (namespace, {{ $controllerKind | lower }}, container, image) (
      k8s_image_availability_exporter_{{ $controllerKind | lower }}_absent == 1
    )
    * on (namespace) group_left()
    max by (namespace) (extended_monitoring_enabled)
  for: 30m
  labels:
    severity_level: "7"
    d8_module: extended-monitoring
    d8_component: image-availability-exporter
  annotations:
    plk_protocol_version: "1"
    plk_markup_format: "markdown"
    plk_create_group_if_not_exists__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    plk_grouped_by__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    description: >
      You should check whether the `{{`{{ $labels.image }}`}}` image is available:
      in the `{{`{{ $labels.namespace }}`}}` Namespace;
      in the {{ $controllerKind }} `{{`{{ $labels.`}}{{ $controllerKind | lower }}{{` }}`}}`
      in the `{{`{{ $labels.container }}`}}` container in the registry.
    summary: The `{{`{{ $labels.image }}`}}` image is missing from the registry.

- alert: {{ $controllerKind }}BadImageFormat
  expr: |
    max by (namespace, {{ $controllerKind | lower }}, container, image) (
      k8s_image_availability_exporter_{{ $controllerKind | lower }}_bad_image_format == 1
    )
    * on (namespace) group_left()
    max by (namespace) (extended_monitoring_enabled)
  for: 30m
  labels:
    severity_level: "7"
    d8_module: extended-monitoring
    d8_component: image-availability-exporter
  annotations:
    plk_protocol_version: "1"
    plk_markup_format: "markdown"
    plk_create_group_if_not_exists__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    plk_grouped_by__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    description: >
      You should check whether the `{{`{{ $labels.image }}`}}` image name is spelled correctly:
      in the `{{`{{ $labels.namespace }}`}}` Namespace;
      in the {{ $controllerKind }} `{{`{{ $labels.`}}{{ $controllerKind | lower }}{{` }}`}}`
      in the `{{`{{ $labels.container }}`}}` container in the registry.
    summary: The `{{`{{ $labels.image }}`}}` image has incorrect name.

- alert: {{ $controllerKind }}RegistryUnavailable
  expr: |
    max by (namespace, {{ $controllerKind | lower }}, container, image) (
      k8s_image_availability_exporter_{{ $controllerKind | lower }}_registry_unavailable == 1
    )
    * on (namespace) group_left()
    max by (namespace) (extended_monitoring_enabled)
  for: 30m
  labels:
    severity_level: "7"
    d8_module: extended-monitoring
    d8_component: image-availability-exporter
  annotations:
    plk_protocol_version: "1"
    plk_markup_format: "markdown"
    plk_create_group_if_not_exists__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    plk_grouped_by__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    description: >
      The container registry is not available for the `{{`{{ $labels.image }}`}}` image:
      in the `{{`{{ $labels.namespace }}`}}` Namespace;
      in the {{ $controllerKind }} `{{`{{ $labels.`}}{{ $controllerKind | lower }}{{` }}`}}`
      in the `{{`{{ $labels.container }}`}}` container in the registry.
    summary: The container registry is not available for the `{{`{{ $labels.image }}`}}` image.

- alert: {{ $controllerKind }}AuthenticationFailure
  expr: |
    max by (namespace, {{ $controllerKind | lower }}, container, image) (
      k8s_image_availability_exporter_{{ $controllerKind | lower }}_authentication_failure == 1
    )
    * on (namespace) group_left()
    max by (namespace) (extended_monitoring_enabled)
  for: 30m
  labels:
    severity_level: "7"
    d8_module: extended-monitoring
    d8_component: image-availability-exporter
  annotations:
    plk_protocol_version: "1"
    plk_markup_format: "markdown"
    plk_create_group_if_not_exists__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    plk_grouped_by__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    description: >
      Unable to login to the container registry using `imagePullSecrets` for the `{{`{{ $labels.image }}`}}` image
      in the `{{`{{ $labels.namespace }}`}}` Namespace;
      in the {{ $controllerKind }} `{{`{{ $labels.`}}{{ $controllerKind | lower }}{{` }}`}}`
      in the `{{`{{ $labels.container }}`}}` container in the registry.
    summary: Unable to login to the container registry using `imagePullSecrets` for the `{{`{{ $labels.image }}`}}` image.

- alert: {{ $controllerKind }}AuthorizationFailure
  expr: |
    max by (namespace, {{ $controllerKind | lower }}, container, image) (
      k8s_image_availability_exporter_{{ $controllerKind | lower }}_authorization_failure == 1
    )
    * on (namespace) group_left()
    max by (namespace) (extended_monitoring_enabled)
  for: 30m
  labels:
    severity_level: "7"
    d8_module: extended-monitoring
    d8_component: image-availability-exporter
  annotations:
    plk_protocol_version: "1"
    plk_markup_format: "markdown"
    plk_create_group_if_not_exists__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    plk_grouped_by__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    description: >
      Insufficient privileges to pull the `{{`{{ $labels.image }}`}}` image using the `imagePullSecrets` specified
      in the `{{`{{ $labels.namespace }}`}}` Namespace;
      in the {{ $controllerKind }} `{{`{{ $labels.`}}{{ $controllerKind | lower }}{{` }}`}}`
      in the `{{`{{ $labels.container }}`}}` container in the registry.
    summary: Insufficient privileges to pull the `{{`{{ $labels.image }}`}}` image using the `imagePullSecrets` specified.

- alert: {{ $controllerKind }}UnknownError
  expr: |
    max by (namespace, {{ $controllerKind | lower }}, container, image) (
      k8s_image_availability_exporter_{{ $controllerKind | lower }}_unknown_error == 1
    )
    * on (namespace) group_left()
    max by (namespace) (extended_monitoring_enabled)
  for: 30m
  labels:
    severity_level: "7"
    d8_module: extended-monitoring
    d8_component: image-availability-exporter
  annotations:
    plk_protocol_version: "1"
    plk_markup_format: "markdown"
    plk_create_group_if_not_exists__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    plk_grouped_by__unavailable_images_in_namespace: "UnavailableImagesInNamespace,namespace={{`{{ $labels.namespace }}`}},prometheus=deckhouse,kubernetes=~kubernetes"
    description: |
      An unknown error occurred for the  `{{`{{ $labels.image }}`}}` image
      in the `{{`{{ $labels.namespace }}`}}` Namespace;
      in the {{ $controllerKind }} `{{`{{ $labels.`}}{{ $controllerKind | lower }} {{` }}`}}`
      in the `{{`{{ $labels.container }}`}}` container in the registry.

      Refer to the exporter logs: `kubectl -n d8-monitoring logs -l app=image-availability-exporter -c image-availability-exporter`
    summary: An unknown error occurred for the  `{{`{{ $labels.image }}`}}` image.
{{- end }}

- name: d8.extended-monitoring.image-availability-exporter.image-checks
  rules:

{{- range list "Deployment" "StatefulSet" "DaemonSet" "CronJob" }}
  {{- include "image-availability-alerts-by-mode" . | nindent 2 }}
{{- end }}
