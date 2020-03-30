#!/bin/bash -e

### Application Dashboard Migration (delete after deploy to all clusters):
###    Delete dashboards by UID, because they have no entry in dashboard_provision table
###    It leads us to difference in state between filesystem and grafana
for f in $(find /frameworks/shell/ -type f -iname "*.sh"); do
  source $f
done

function __config__() {
  cat << EOF
    configVersion: v1
    onStartup: 1
EOF
}

function __main__() {
  # Delete all dashboard that have same uids as ones from monitoring-application module
  #   and have no entry in dashboard_provisioning table
  cat <<EOF | sqlite3 /var/lib/grafana-storage/grafana.db
  DELETE FROM dashboard WHERE id IN (
    SELECT d1.id FROM dashboard d1
    INNER JOIN dashboard d2
    WHERE d2.is_folder = true
      AND d2.slug = "applications"
      AND d1.folder_id = d2.id
      AND d1.uid IN ("DV0yeDkmz", "vS0Tevziz", "DJKmY0Kmk", "BeAQB3diz", "pZO_eDzmz",
                     "mlRD4Simk", "SwCwV7qkz", "ktDL6Dzik", "CFIjvxzik", "7v6fcOkmz",
                     "YFjpuvzik")
      AND d1.id NOT IN (SELECT dashboard_id FROM dashboard_provisioning)
  );
EOF
}

hook::run "$@"
