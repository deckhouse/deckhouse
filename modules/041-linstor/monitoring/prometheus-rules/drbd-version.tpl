- name: kubernetes.drbd.device_state
  rules:
    - alert: D8DRBDVersionIsOutdated
      expr: drbd_version{kmod!="{{ $.Values.linstor.internal.drbdVersion }}"}
      for: 5m
      labels:
        severity_level: "9"
        tier: cluster
      annotations:
        plk_markup_format: "markdown"
        plk_protocol_version: "1"
        plk_create_group_if_not_exists__d8_drbd_device_health: "D8DrbdVersion,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes"
        plk_grouped_by__d8_drbd_device_health: "D8DrbdVersion,tier=~tier,prometheus=deckhouse,kubernetes=~kubernetes"
        summary: DRBD version is outdated
        description: |
          LINSTOR node `{{"{{ $labels.node }}"}}` has outdated DRBD version

          in use: `{{"{{ $labels.kmod }}"}}`
          expected: `{{ $.Values.linstor.internal.drbdVersion }}`

          The recommended course of action:
          - reboot the node
