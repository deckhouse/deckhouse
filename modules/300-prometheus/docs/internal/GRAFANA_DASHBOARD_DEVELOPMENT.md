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

1. Find all ranges and replace them with `$__interval_sx4`:

   ```shell
   for dashboard in *.json; do
     for range in $(grep '\[[0-9]\+[a-z]\]' $dashboard | sed 's/.*\(\[[0-9][a-z]\]\).*/\1/g' | sort | uniq); do
       sed -e 's/\['$range'\]/[$__interval_sx4]/g' -i $dashboard
     done
   done
   ```

2. Replace `irate` with `rate`:

   ```shell
   sed 's/irate(/rate(/g' -i *.json
   ```

3. Replace `Resolution` with `1/1`:

   ```shell
   sed 's/"intervalFactor":\s[0-9]/"intervalFactor": 1/' -i *.json
   ```

4. Get rid of `Min Step`:

   ```shell
   sed '/"interval":/d' -i *.json
   ```

5. Replace all graphs with `Staircase` (you will need to edit `Stack` + `Percent` graphs manually - switch them to `Bars`):

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
2. **use `$__interval_sx4` as the range for the range vectors**;
3. **use Resolution 1/1 in al cases**;
4. **never set the Min step**;
5. **use `$__interval_sx3` as the range for the range vectors in the avg/max/min_over_time functions**;

![Data accuracy and granularity](/docs/documentation/images/300-prometheus/grafana_accuracy.jpg)

{% offtopic title="Reasons and details" %}
  <ul dir="auto">
    <li>You can specify <code>step</code> in the Prometheus's API request. Suppose we have three hours of data and set a <code>step</code> of 30 seconds. In this case, we will get 360 data points (3 hours *60 minutes* 2 points per minute), and they can easily fit on the graph. Now suppose we have data for 24 hours. In this case, the 30-second step does not make any sense since you cannot fit 2880 data points on a screen (unless, of course, you have a 4K monitor - but still, each data point will have the size of a pixel, and the human eye cannot discern so much tightly packed information). To solve this problem, Grafana implements a tricky mechanism to auto-determine the step size. It works as follows:
    <ul>
      <li>Grafana uses the size of the graph (whether it occupies the entire screen, 1/2, 1/4 of the screen, etc.), the size of the browser window, and the screen resolution to calculate how many points can be shown on the screen.</li>
      <li>Next, Grafana divides the selected browsing period by the number of points that can be shown to get the "minimum viable step". Thus, for the screen that can fit 800 data points, it gets the following ratios:
        <ul>
          <li>30 minutes — 2.25 seconds,</li>
          <li>3 hours — 13.5 seconds,</li>
          <li>24 hours — 108 seconds,</li>
          <li>7 days — 756 seconds.</li>
        </ul>
      </li>
      <li>Next, Grafana turns to the <code>Min step</code> parameter and makes the minimum data point <code>step</code> equal to the <code>Min step</code> (if specified). Note that the <code>Min step</code> parameter can be specified globally (in the data source settings) and for each query in the panel. In our case, the <code>Min step</code>  is set globally. It corresponds to the <code>scrape_interval</code> for Prometheus (the <code>scrape_interval</code> for the main one is 30 seconds). In the end, Grafana gets the following values (taking into account the above restriction):
        <ul>
          <li>30 minutes — 30 seconds (instead of 2.25),</li>
          <li>3 hours — 30 seconds (instead of 13.5 seconds),</li>
          <li>24 hours — 108 seconds,</li>
          <li>7 days — 756 seconds.</li>
        </ul>
      </li>
      <li>Next, Grafana rounds the resulting values (to 5/15/30 seconds, 1/5/15 minutes, etc.) and gets the following:
        <ul>
          <li>30 minutes — 30 seconds,</li>
          <li>3 hours — 30 seconds,</li>
          <li>24 hours — 2 minutes,</li>
          <li>7 days — 10 minutes.</li>
        </ul>
      </li>
      <li>Next, Grafana refers to the Resolution parameter of the panel (it can be set to 1/1, 1/2, ..., 1/10) and increases the step according to it (twice for 1/2, in ten times for 1/10).</li>
    </ul>
  </li>
  <li>Most of the Prometheus data are stored in counters (not gauges), so you need to use the <code>rate</code> or <code>irate</code> function to get the current value. And that is where the problem begins.
    <ul>
      <li>The <code>rate</code> and <code>irate</code> functions use range vectors, but which range to pass? Grafana has a built-in (and ready-to-use) <code>$__interval</code> variable that stores the step that will be passed to Prometheus.</li>
      <li>But there must be al least two points in the range vector for the <code>rate</code> and <code>irate</code> functions to work (which is logical). However, a range vector for 30 seconds with a <code>scrape_interval</code> set to 30 seconds will contain only one point, and <code>rate</code>/<code>irate</code> will be useless in this case. And at this point, you may opt for any of the following WRONG approaches:
        <ul>
          <li>Set the Resolution to 1/2 for all queries so that the <code>$__interval</code> variable equals 2 x <code>Min step</code> for any interval. It helps — the graphs work as expected. The downside is that they are always half as detailed as they could otherwise have been.</li>
          <li>Set the Min step equal to two <code>scrape_intervals</code> This approach is somewhat better but has the same downsides.</li>
          <li>Use the <code>irate</code> function and pass a range vector for 1h (or any other value known to be larger than the period). This approach is the most deceptive. In this case, graphs are displayed accurately as long as the <code>step</code> is less or equal to the <code>scrape_interval</code>. However, if the  <code>step</code> is bigger than the <code>scrape_interval</code> (as is the case with 24h and 7d periods), the graph becomes utterly misleading: instead of displaying the rate for the entire step, it shows the rate for the last <code>scrape_interval</code> at each point. In other words, when browsing data for the 7 days, you see the CPU usage over the last 30 seconds of each 10-minute interval instead of the average usage for a (<code>step</code>) (10 minutes). Thus, you have no idea of what has happened to the CPU usage in the remaining 9 minutes and 30 seconds!</li>
        </ul>
      </li>
      <li>To solve this problem, we have added the <code>$__interval_sx4</code> (rv = range vector) variable to Grafana. This variable equals <code>max($__interval, scrape_interval * 2)</code>. It is similar to <code>$__interval</code> while its minimum value cannot be less than the period containing at least two points in the range vector (<a href="https://github.com/grafana/grafana/issues/11451" rel="nofollow noreferrer noopener" target="_blank">here is the corresponding Grafana issue</a>). And that completely solves the problem!</li>
    </ul>
  </li>
</ul>
{% endofftopic %}

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
