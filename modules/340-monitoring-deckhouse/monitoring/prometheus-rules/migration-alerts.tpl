- name: d8.migration-alerts
  rules:
  - alert: MigrationRequiredFromRBDInTreeProvisionerToCSIDriver
    expr: |
      kube_storageclass_info{provisioner="kubernetes.io/rbd"}
    for: "10m"
    labels:
      tier: cluster
      severity_level: "9"
    annotations:
      plk_markup_format: markdown
      plk_protocol_version: "1"
      summary: StorageClass `{{"{{ $labels.storageclass }}"}}` is using a deprecated RBD provisioner.
      description: |
        To resolve this issue, migrate volumes to the Ceph CSI driver using the `rbd-in-tree-to-ceph-csi-migration-helper.sh` script available at `https://github.com/deckhouse/deckhouse/blob/{{ $.Values.global.deckhouseVersion }}/modules/031-ceph-csi/tools/rbd-in-tree-to-ceph-csi-migration-helper.sh`.

        For details on volume migration, refer to the migration guide available at `https://github.com/deckhouse/deckhouse/blob/{{ $.Values.global.deckhouseVersion }}/modules/031-ceph-csi/docs/internal/INTREE_MIGRATION.md`.
