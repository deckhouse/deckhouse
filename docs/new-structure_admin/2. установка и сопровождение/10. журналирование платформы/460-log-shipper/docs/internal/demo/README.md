# Steps

## Install Loki

See [**loki.txt**](./loki.txt) for more details.

## Add all-pods pipeline

* Apply [**all-pods-pipeline**](./all-pods-pipeline.yaml) manifests to the cluster.
* Show debug information in vector pods.

## Add Loki datasource to Grafana

* Apply [**grafana-additional-datasource**](./grafana-additional-datasource.yaml) manifests to the cluster.
* Show Grafana `Explore` page and query pods logs: `{log_shipper_source="all-pods"}`.

## Enrich audit logs

* Create [**audit-policy**](./audit-policy.yaml) secret.
* Enable corresponding control plane option in the cluster (you can find a link to the doc in the [**audit-policy.yaml**](./audit-policy.yaml)).

## Add audit-logs pipeline

* Apply [**audit-logs-pipeline**](./audit-logs-pipeline.yaml) manifests to the cluster.
* Show debug information in vector pods.
* Show Grafana `Explore` page and query audit logs: `{log_shipper_source="audit-logs"}`.
* Create a Grafana Dashboard.

# Cleaning

Execute [**clean_up.sh**](./clean_up.sh)
