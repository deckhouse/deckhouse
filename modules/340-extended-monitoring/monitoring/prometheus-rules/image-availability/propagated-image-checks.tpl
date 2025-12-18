{{- define "image-availability-alerts-by-mode" }}
{{- $controllerKind := . }}

{{/* TODO: Make a single alert for all controllers. */}}

- alert: {{ $controllerKind }}ImageAbsent
  expr: |
    max by (namespace, name, container, image) (
      k8s_image_availability_exporter_absent{kind={{ $controllerKind | lower | quote }}} == 1
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
    summary: The `{{`{{ $labels.image }}`}}` image is missing from the registry.
    description: |
      Deckhouse has detected that the `{{`{{ $labels.image }}`}}` image is missing from the container registry.

      To resolve this issue, check whether the `{{`{{ $labels.image }}`}}` image is available in the following sources:

      - The `{{`{{ $labels.namespace }}`}}` namespace.
      - The {{ $controllerKind }} `{{`{{ $labels.name }}`}}`.
      - The `{{`{{ $labels.container }}`}}` container in the registry.

- alert: {{ $controllerKind }}BadImageFormat
  expr: |
    max by (namespace, name, container, image) (
      k8s_image_availability_exporter_bad_image_format{kind={{ $controllerKind | lower | quote }}} == 1
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
    summary: The `{{`{{ $labels.image }}`}}` image name is incorrect.
    description: |
      Deckhouse has detected that the `{{`{{ $labels.image }}`}}` image name is incorrect.

      To resolve this issue, check that the `{{`{{ $labels.image }}`}}` image name is spelled correctly in the following sources:

      - The `{{`{{ $labels.namespace }}`}}` namespace.
      - The {{ $controllerKind }} `{{`{{ $labels.name }}`}}`.
      - The `{{`{{ $labels.container }}`}}` container in the registry.

- alert: {{ $controllerKind }}RegistryUnavailable
  expr: |
    max by (namespace, name, container, image) (
      k8s_image_availability_exporter_registry_unavailable{kind={{ $controllerKind | lower | quote }}} == 1
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
    summary: The container registry is not available for the `{{`{{ $labels.image }}`}}` image.
    description: |
      Deckhouse has detected that the container registry is not available for the `{{`{{ $labels.image }}`}}` image.

      To resolve this issue, investigate the possible causes in the following sources:

      - The `{{`{{ $labels.namespace }}`}}` namespace.
      - The {{ $controllerKind }} `{{`{{ $labels.name }}`}}`.
      - The `{{`{{ $labels.container }}`}}` container in the registry.

- alert: {{ $controllerKind }}AuthenticationFailure
  expr: |
    max by (namespace, name, container, image) (
      k8s_image_availability_exporter_authentication_failure{kind={{ $controllerKind | lower | quote }}} == 1
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
    summary: Unable to log in to the container registry using `imagePullSecrets` for the `{{`{{ $labels.image }}`}}` image.
    description: |
      Deckhouse was unable to log in to the container registry using `imagePullSecrets` for the `{{`{{ $labels.image }}`}}` image.

      To resolve this issue, investigate the possible causes in the following sources:

      - The `{{`{{ $labels.namespace }}`}}` namespace.
      - The {{ $controllerKind }} `{{`{{ $labels.name }}`}}`.
      - The `{{`{{ $labels.container }}`}}` container in the registry.

- alert: {{ $controllerKind }}AuthorizationFailure
  expr: |
    max by (namespace, name, container, image) (
      k8s_image_availability_exporter_authorization_failure{kind={{ $controllerKind | lower | quote }}} == 1
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
    summary: Insufficient privileges to pull the `{{`{{ $labels.image }}`}}` image using the specified `imagePullSecrets`.
    description: |
      Deckhouse has insufficient privileges to pull the `{{`{{ $labels.image }}`}}` image using the specified `imagePullSecrets`.

      To resolve this issue, investigate the possible causes in the following sources:

      - The `{{`{{ $labels.namespace }}`}}` namespace.
      - The {{ $controllerKind }} `{{`{{ $labels.name }}`}}`.
      - The `{{`{{ $labels.container }}`}}` container in the registry.

- alert: {{ $controllerKind }}UnknownError
  expr: |
    max by (namespace, name, container, image) (
      k8s_image_availability_exporter_unknown_error{kind={{ $controllerKind | lower | quote }}} == 1
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
    summary: An unknown error occurred with the `{{`{{ $labels.image }}`}}` image.
    description: |
      Deckhouse has detected an unknown error with the `{{`{{ $labels.image }}`}}` image in the following sources:

      - The `{{`{{ $labels.namespace }}`}}` namespace.
      - The {{ $controllerKind }} `{{`{{ $labels.name }}`}}`.
      - The `{{`{{ $labels.container }}`}}` container in the registry.

      To resolve this issue, review the exporter logs:

      ```bash
      d8 k -n d8-monitoring logs -l app=image-availability-exporter -c image-availability-exporter
      ```

{{- end }}

- name: d8.extended-monitoring.image-availability-exporter.image-checks
  rules:

{{- range list "Deployment" "StatefulSet" "DaemonSet" "CronJob" }}
  {{- include "image-availability-alerts-by-mode" . | nindent 2 }}
{{- end }}
