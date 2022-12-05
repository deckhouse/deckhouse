# Steps

## Install VictoriaMetrics

See [**victoria-metrics.txt**](./victoria-metrics.txt) for more details.

## Add PrometheusRemoteWrite

* Apply [**prometheus-remote-write**](./prometheus-remote-write.yaml) manifests to the cluster.

## Add VictoriaMetrics datasource to Grafana

* Apply [**grafana-additional-datasource**](./grafana-additional-datasource.yaml) manifests to the cluster.
* Show Grafana `Explore` page and query metrics from VictoriaMetrics.

## Add an alert that is always firing

* Apply [**custom-prometheus-rule**](./custom-prometheus-rule.yaml) manifests to the cluster.
* Show the alerts page in Prometheus.

## Add a brand new Grafana dashboard

* Apply [**grafana-dashboard-definition**](./grafana-dashboard-definition.yaml) manifests to the cluster.
* Show the dashboards page in Grafana.

## Deploy Custom alertmanager

* Apply [**alertmanager**](./alertmanager.yaml) manifests to the cluster.
* Show the starting page of Alertmanager.

# Cleaning

Execute [**clean_up.sh**](./clean_up.sh)
