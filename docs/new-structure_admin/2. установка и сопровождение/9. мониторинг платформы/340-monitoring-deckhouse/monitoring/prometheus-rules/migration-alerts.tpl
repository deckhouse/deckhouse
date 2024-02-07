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
      summary: Storage class `{{"{{ $labels.storageclass }}"}}` uses the deprecated rbd provisioner. It is necessary to migrate the volumes to the Ceph CSI driver.
      description: |
        To migrate volumes use this script https://github.com/deckhouse/deckhouse/blob/{{ $.Values.global.deckhouseVersion }}/modules/031-ceph-csi/tools/rbd-in-tree-to-ceph-csi-migration-helper.sh
        A description of how the migration is performed can be found here https://github.com/deckhouse/deckhouse/blob/{{ $.Values.global.deckhouseVersion }}/modules/031-ceph-csi/docs/internal/INTREE_MIGRATION.md
