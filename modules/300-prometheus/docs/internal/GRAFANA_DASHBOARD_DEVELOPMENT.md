---
title: "Prometheus monitoring: making Grafana graphs"
type:
  - instruction
search: making Grafana graphs
---

1. The [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) custom resource allows you to create Grafana dashboards. The [shell-operator](https://github.com/flant/shell-operator) sidecar container runs along with the main container in the Grafana Pod and monitors that resource. The hook creates/deletes/edits the specific dashboard in response to a corresponding event using some special mechanism.
2. In modules, dashboard manifests are located at `<module_root>/monitoring/grafana-dashboards/`. They are automatically converted to [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) CRs, while:
   * each subdirectory in the above directory corresponds to a Folder in Grafana,
   * and each file corresponds to a Dashboard in Grafana.
3. If you need to add a new Folder, just create a directory in the `grafana-dashboards/` and add at least one JSON manifest to it.
4. You do not need to edit Dashboard files directly (unless you want to do a minor quick fix). Instead:
   * Open the Dashboard in Grafana:
     * if the Dashboard already exists, open it and click the ["Make editable" button](/docs/documentation/images/300-prometheus/grafana_make_editable.jpg);
     * if the Dashboard is a new one, then create it (in any Folder — it will be moved to the folder that corresponds to the repository directory);
     * if the Dashboard is a third-party one, import it to Grafana (via the "Import" button).
   * Now you can edit your Dashboard until you are happy with it. Do not forget to click the "Save" button in Grafana periodically to keep your changes (just in case).
   * [Export the Dashboard to JSON](/docs/documentation/images/300-prometheus/grafana_export.jpg) and save it to a file (the new or existing one).
5. You can rename the Dashboard, rename the file, move the file from one Folder to another — everything will be configured automatically.
6. Note that system Dashboards must be stored in `d8`-prefixed `GrafanaDashboardDefinition` custom resources.

## How do I quickly optimize a third-party Dashboard?

1. Replace `irate` with `rate`:

   ```shell
   sed 's/irate(/rate(/g' -i *.json
   ```

2. Replace `Resolution` with `1/1`:

   ```shell
   sed 's/"intervalFactor":\s[0-9]/"intervalFactor": 1/' -i *.json
   ```

3. Get rid of `Min Step`:

   ```shell
   sed '/"interval":/d' -i *.json
   ```

4. Replace all graphs with `Staircase` (you will need to edit `Stack` + `Percent` graphs manually - switch them to `Bars`):

   ```shell
   sed 's/"steppedLine": false/"steppedLine": true/' -i *.json
   ```

## Best practices

### Save the Dashboard's uid

Save the uid when making any changes to the Dashboard (including renaming it and moving it between folders). Usually, you can find it at the end of the JSON file. In this case, all the existing links will continue to work. Note that no special actions are required - just do not change it intentionally.

### Monitor JSON files for changes

We do not recommend editing JSON files directly (there are more convenient ways to do this). However, after making changes to the Grafana interface and uploading the JSON file, you should take a careful look at what changes were made to it to make sure that you haven't messed anything up accidentally. (You can use `git add -p` & diffs in merge requests to check what changes were made to the file - it's your choice).

When making changes to the complex Dashboards that use templates, try to edit them where they were created (usually at <https://prometheus.kube.domain.my/>) and keep the Variables intact. Thus, you will avoid unnecessary changes to the "dynamic data" in MRs.

### How to "hide" the dashboard (or part of the dashboard) if data are lacking?

Do not try to generate Grafana Dashboard using Helm. A better approach is to create an Issue in Grafana requesting the required functionality.

## Dashboard requirements

### The job must be specified explicitly

You must always [explicitly specify the job](prometheus_rules_development.html#specify-the-job-explicitly-in-all-cases) to ensure zero conflicts in the metric names (the same applies to the development of Prometheus rules).

### The user must be able to choose the Prometheus instance

Our Grafanas may have several Prometheus instances available (with different data granularity and storage periods). That means that the user must be able to choose the Prometheus server. To do this, you need to:
* [create](/docs/documentation/images/300-prometheus/grafana_ds_prometheus_variable.jpg) a `$ds_prometheus` variable;
* [set](/docs/documentation/images/300-prometheus/grafana_ds_prometheus_select_in_panel.jpg) the `$ds_prometheus` as the data source in each panel (instead of setting the specific Prometheus instance).

### Graph Tooltip must be in Shared crosshair mode

One of the crucial features that graphs provide is the ability to analyze correlations visually. However, to perform such an analysis, you need to compare the same point on the time axis on different graphs. You can easily do that if graphs have the same size and are positioned under each other (but that doesn't look lovely). The problem is, in some cases, graphs cannot have the same size.

Thus, always [use](/docs/documentation/images/300-prometheus/grafana_graph_tooltip.jpg) the Graph Tooltip in the Shared crosshair mode. It helps much in analyzing correlations visually: hover the mouse pointer over one of the graphs, and a bar will appear on all other graphs at the same point on the time axis.

We do not recommend using the more sophisticated Shared tooltip mode since it overloads the user with too much information.

### The units are crucial

A graph without a unit of measurement is always misleading! What does the graph shows - rpm or rps, bits or bytes? The unit of measurement put on a graph makes it possible for the user to understand what the graph shows. In Grafana, you can explicitly specify the unit of measurement (and you should do that!).

In addition, it is better to use typical units of measurement such as:
* The data transfer rate is usually measured in bits per second, not in bytes per second;
* The number of operations is usually specified on a per-second basis (and not on a per-minute one - iops, rps, etc.);
* The volume of data is always measured in bytes, not bits.

### Data accuracy and granularity

Granted, in some cases, the level of detail makes it difficult to track global trends. However, the opposite is true much more often — due to insufficient detail, part of the data required for analysis is not displayed. To preserve the accuracy of data:
1. **always use the `rate` function instead of `irate`**;
2. **use `$__rate_interval` as the range for the range vectors**;
3. **use Resolution 1/1 in al cases**;
4. **never set the Min step**;
5. **use `$__range` as the range for the range vectors in the avg/max/min_over_time functions**;

#### The absence of data must be obvious

Grafana provides three modes for displaying the Null Value (no data).
* You should never use the `connected` mode since it is highly misleading!
* `Null` should be used in all cases except for stacking since it clearly shows that there are no data.
* The usage of the `null as zero` mode is recommended for stacking (otherwise, all the metrics will be lost if any one of them has a null value).

| connected               | null          | null as zero                  |
|-------------------------|---------------|-------------------------------|
| ![connected][connected] | ![null][null] | ![null as zero][null as zero] |

[connected]: /docs/documentation/images/300-prometheus/grafana_null_value_connected.jpg
[null]: /docs/documentation/images/300-prometheus/grafana_null_value_null.jpg
[null as zero]: /docs/documentation/images/300-prometheus/grafana_null_value_as_zero.jpg

#### The precision should be consistent with goals

* There is no reason to display values precisely to five decimal places if your error rate is 10%. The typical user will notice only the first two-three most significant digits anyway.
* If you have tens of thousands (or even millions) of requests over 3 hours (the default period for a Dashboard), does the precise value make sense? Maybe, the order of the number of requests is sufficient in this case?

Pay attention: the precision of the value must match the goals of using the indicator.

#### Display data for the last 3 hours by default (auto-refresh every 30 seconds)

The three-hour display period is optimal (and should be used as default) since this is the maximum one for a fully detailed view.
* With the `scrape_interval` set to `30s` (the most commonly used value), three hours of data fully fit even on graphs with a size of 1/4 of the screen width (meaning all 30-second data points are displayed) - no approximation or conversion is needed.
* A larger scale (the smaller display period) does not make sense since it does not increase the detail but narrows the visible time range.
* A smaller scale (the larger display period) reduces the details, and the approximation comes into play.

It makes sense to set automatic updates every 30 seconds since Prometheus scrapes the data every 30 seconds. Thus, new data flow to it at regular intervals.

#### Before pushing new dashboards, you have to remove all references to the existing domains

Before pushing changes to the repository, you have to make sure that there are no domains in the JSON file that may have been imported from Grafana.

An example of a script to delete these domains:

```shell
listOfDomains="
google.com
mycompany.com
"

listOfDashboards=$(find dashboards/* -name "*json")

for dashboard in $listOfDashboards; do
  for domain in $listOfDomains; do
    sed -i -E  "s/([^\"]+$domain)/example.com/g" $dashboard
  done
done
```

### TODO

* Make use of $__range and instant query to calculate data for the display period (usually used for singlestats);
* Make the legends of the same width so that the graphs can be displayed one below the other.
* Display a zero for the Y-axis.
* Display the upper bound of the Y-axis for 0-100% graphs.
* How to plot percentage charts (bars, instead of stepped lines).
* A trick with using stacking and showing Total.
* Set "On time range change" when the variable gets values that may change (e.g., the Pod names).
* Make CPU colors to be the same as in okmeter.
* What drawing mode to use and what is good about the staircase mode.
