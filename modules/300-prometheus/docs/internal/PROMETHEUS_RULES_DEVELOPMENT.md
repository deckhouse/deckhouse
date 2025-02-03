---
title: "Developing Prometheus rules"
type:
  - instruction
search: Developing Prometheus rules, prometheus alerting rules
---

## General information

* The rules in Prometheus are divided into two types:
  * recording rules (the [official documentation](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)) — allow you to precompute the PromQL expression and save the result to a new metric (it is useful if you need to speed up Grafana or the calculation of other rules).
  * alerting rules (the [official documentation](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)) — allow you to define alert conditions based on the result of the PromQL expression.
* All the rules are divided according to the module and are located in the [monitoring/prometheus-rules](https://github.com/deckhouse/deckhouse/tree/main/modules/300-prometheus/monitoring/prometheus-rules/)`. The rules are divided into three categories:
  * `coreos` stores rules originating from the prometheus-operator (some of them are modified by us);
  * `kubernetes` stores our rules related to Kubernetes monitoring (the platform — control plane, NGINX Ingress, Prometheus, etc) and monitoring of objects in Kubernetes (Pods, CronJobs, disk space, etc.);
  * `applications` stores rules for monitoring applications (e.g., redis, mongo, etc.).
* Changes to these files (including the creation of new ones) should be automatically shown on the `/prometheus/rules` page (you need to wait about a minute after deckhouse is deployed so that Prometheus Operator and other tools do their work).
* Here is how you can troubleshoot the problem if your changes are not shown (for more information, see the documentation of the [Prometheus Operator](../../modules/operator-prometheus/) module):
  * Check that your changes are present in the ConfigMap in Kubernetes:
    * `kubectl -n d8-monitoring get prometheusrule/prometheus-rules-<DIRECTORY NAME> -o yaml`
    * If no changes are shown, you need to check that deckhouse is deployed successfully:
      * `helm -n d8-system ls` — prometheus must have the DEPLOYED status
      * `kubectl -n d8s-system logs deploy/deckhouse -f` — the log should not contain errors
  * Check that prometheus-config-reloader has noticed your changes:
    * The output of the `kubectl -n d8-monitoring logs prometheus-main-0 prometheus-config-reloader -f` command must contain an appropriate message:

      ```text
      ts=2018-04-12T12:10:24Z caller=main.go:244 component=volume-watcher msg="ConfigMap modified."
      ts=2018-04-12T12:10:24Z caller=main.go:204 component=volume-watcher msg="Updating rule files..."
      ts=2018-04-12T12:10:24Z caller=main.go:209 component=volume-watcher msg="Rule files updated."
      ```

    * If `prometheus-config-reloader` doesn't notice any changes, check prometheus-operator:
      * `kubectl -n d8-operator-prometheus get pod` — verify the the Pod is running
      * `kubectl -n d8-operator-prometheus logs -f deploy/prometheus-operator --tail=50` — verify that the log does not contain errors
    * If `prometheus-config-reloader` cannot reload prometheus, then there is an error in the rules and you need to analyze the Prometheus log:
      * `kubectl -n d8-monitoring logs prometheus-main-0 prometheus -f`
    * **Caution!** Note that sometimes `prometheus-config-reloader` "hangs" at some error and overlooks new changes - it keeps trying to reload Prometheus using the old erroneous config. In this case, the only thing you can do is exec to the Pod and kill the `prometheus-config-reloader` process (Kubernetes will restart the container).

## Best practices

### Adhere to the standard for naming rule groups

The rules in Prometheus are divided into groups (take a look at any rules file as an example). The group must be named according to the following format: `<directory name>.<file name without extension>.<group name>`. The name of the group can be omitted. For example:
* there are three groups in `kubernetes/nginx-ingress.yaml`: `kubernetes.nginx-ingress.overview`, `kubernetes.nginx-ingress.details`, and `kubernetes.nginx-ingress.controller`;
* and there is just one group in `applications/redis.yaml`: `applications.redis`.

### Specify the job explicitly in all cases

The name of the metric (even if it seems unique), might cease to be unique at any time — someone can add the custom application with metrics with the same name to one of the clusters. In this case, your rules will fail. Happily, the job label is controlled very strictly (thanks to servicemonitors), which is why the `metric's name` + `job` pair is guaranteed to be unique. Therefore, **always add the job name** to all requests for all rules!

For example, the `nginx_filterzone_responses_total` metric is the standard one ([nginx-vts-exporter](https://github.com/hnlq715/nginx-vts-exporter) exports it). If you do not specify the job name explicitly, then any custom application that exports these metrics will break all ingress-nginx graphs and alerts.

```text
sum(nginx_filterzone_responses_total{job="nginx-ingress-controller", server_zone="request_time_hist"}) by (job, namespace, scheme, server_name)
                                     ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                     ensures that there will be no conflicts
```

### Use `irate(foo[1h])` when converting counter to gauge

In some situations a counter cannot be used and you need a ready-made gauge metric. For example, you need to expose the 90th percentile of the max traffic for the last three hours, while the counter represents the traffic. In this case, you need to make a pre-calculated metric and store that gauge in it.

In Grafana, when plotting counter-based graphs, we use `rate[<scrape_interval> * 2]` to get the fully detailed view. Since `scrape_interval` may vary depending on the Prometheus instance, there is a dedicated `$__interval_rv` variable in Grafana (we added it to Grafana ourselves). It contains a double `scrape_interval`. But what about Prometheus? We cannot set 60s intervals in all rules since these rules won't work for Prometheus instances with `scrape_interval` greater than 30s (less than two data points fall into the range vector).

Happily, the solution is very simple. You just need to use `irate(foo[1h])` in all cases. The thing is that:
* The rules are evaluated every `evaluation_interval`,  but we can be sure that it equals to `scrape_interval` in our installations;
* `irate(foo[1h])` returns the rate for the last two data points (i.e., the rate is based on the last two points);
* We can be sure that the `scrape_interval` greater than 30m won't be used (this interval is rare and doesn't have any practical sense).

Thus, it turns out that `irate(foo[1h])` in the Prometheus rules is equivalent to `rate[$__interval_rv]` in Grafana (see "Reasons and details" in the "[Data accuracy and granularity](grafana_dashboard_development.html#data-accuracy-and-granularity)" section of the Grafana dashboard development documentation).

### Don't try to generate rules using Helm

The thing is that Prometheus rules are based on programmable logic, just like Helm Chart templates. Defining some programmable logic using another one is called metaprogramming, and you should not misuse it if you don't have a perfect reason to do so.

For example, if you want to allow for overriding the threshold value in an alert:
* Create the following query: `foo_value > scalar(foo_threshold or vector(5))`.
  * if the `foo_threshold` metric is defined, the `foo_value` be compared with it,
  * if the `foo_threshold` metric is not defined, the `foo_value` will be compared with 5.
* In this case, you can create a custom rule in the appropriate cluster that returns the `foo_threshold` metric with the required threshold value.

## Alerting rules severity

To set the severity level of the alerting rule, use the `severity_level` label (or a pair of `impact` + `likehood` labels). For example:

```yaml
spec:
  groups:
  - name: custom.sentry.exporter-is-down
    rules:
    - alert: SentryExporterDown
      annotations:
        description: |-
          Prometheus cannot connect to the metric exporter for two minutes.
          Notify the client's team and the client.
        plk_markup_format: markdown
        plk_protocol_version: "1"
        summary: Sentry metrics are not coming in
      expr: absent(up{job="custom-sentry"} == 1)
      for: 2m
      labels:
        severity_level: "1"
```

In this case, the polk service will display an alert with the S1 criticality level.
You can do the same using `impact` and `likelihood` pair of labels:

```yaml
      labels:
        impact: deadly
        likehood: certain
```

We recommend using the second method (with the `impact` and `likelihood` labels) if the incident can be described in terms of possible harmful consequences and their probability. For example:
* Suppose one of the four disks in the RAID5 array fails (and this array is used to store the database).
  * Failure of another disk will lead to the entire array failure and data loss; thus, `impact: deadly`.
  * The probability of the second disk failing right after the first one is relatively low; however, it is still possible; thus, `likehood: possible`.
  * Two of these labels combined give the S3 criticality level.

Click the criticality icon for any incident in Polk to see the level/label matrix.
