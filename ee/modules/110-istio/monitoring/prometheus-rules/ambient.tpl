{{- if not .Values.istio.ambient.enabled }}
- name: d8.istio.ambient
  rules:
    - alert: D8IstioActiveWaypointsWithAmbientDisabled
      expr: |
        max by (namespace, deployment, waypoint_instance) (
          label_replace(
            kube_deployment_labels{
              label_app="d8-waypoint",
              label_heritage="deckhouse",
              label_istio_deckhouse_io_waypoint_instance!="",
              label_gateway_istio_io_managed="istio.io-mesh-controller"
            },
            "waypoint_instance",
            "$1",
            "label_istio_deckhouse_io_waypoint_instance",
            "(.+)"
          )
          * on (namespace, deployment) group_left()
            (kube_deployment_spec_replicas > 0)
        )
      for: 5m
      labels:
        severity_level: "6"
        tier: cluster
      annotations:
        plk_markup_format: markdown
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_istio_ambient_misconfigurations: D8IstioAmbientMisconfigurations,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
        plk_grouped_by__d8_istio_ambient_misconfigurations: D8IstioAmbientMisconfigurations,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes
        summary: Active Istio waypoint exists while ambient mesh is disabled.
        description: |
          Deckhouse has detected an active waypoint Deployment `{{"{{$labels.deployment}}"}}` in namespace `{{"{{$labels.namespace}}"}}`, created for WaypointInstance `{{"{{$labels.waypoint_instance}}"}}`, while ambient mesh is disabled.

          This usually happens if ambient mesh was enabled, a WaypointInstance was created, and then ambient mesh was disabled. In this state the waypoint-controller is not running, so it cannot reconcile or clean up WaypointInstance resources.

          To inspect affected resources, run:

          ```bash
          d8 k -n {{"{{$labels.namespace}}"}} get waypointinstance {{"{{$labels.waypoint_instance}}"}} -o yaml
          d8 k -n {{"{{$labels.namespace}}"}} get deploy,svc,sa,gateway,pdb,hpa,vpa -l istio.deckhouse.io/waypoint-instance={{"{{$labels.waypoint_instance}}"}}
          ```

          Recommended remediation:

          1. Temporarily enable ambient mesh.
          2. Wait until waypoint-controller starts.
          3. Delete the affected WaypointInstance or change configuration as needed.
          4. Wait until managed waypoint resources are reconciled as needed or removed.
          5. Disable ambient mesh again.

          If ambient mesh cannot be re-enabled, manually remove the managed waypoint resources and then remove the WaypointInstance finalizer only after verifying that no managed resources remain.
{{- end }}
