/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"reflect"
	"testing"
)

func TestValidateGrafanaDashboardFile(t *testing.T) {
	// Dashboard with deprecated components
	in := `
{
  "annotations": {
    "list": [
      {
        "builtIn": 1,
        "datasource": {
          "type": "grafana",
          "uid": "-- Grafana --"
        },
        "enable": true,
        "hide": true,
        "iconColor": "rgba(0, 211, 255, 1)",
        "name": "Annotations & Alerts",
        "target": {
          "limit": 100,
          "matchAny": false,
          "tags": [],
          "type": "dashboard"
        },
        "type": "dashboard"
      }
    ]
  },
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 0,
  "id": 78,
  "links": [],
  "liveNow": false,
  "panels": [
    {
      "alert": {
        "alertRuleTags": {},
        "conditions": [
          {
            "evaluator": {
              "params": [
                null
              ],
              "type": "gt"
            },
            "operator": {
              "type": "and"
            },
            "query": {
              "params": [
                "A",
                "5m",
                "now"
              ]
            },
            "reducer": {
              "params": [],
              "type": "avg"
            },
            "type": "query"
          }
        ],
        "executionErrorState": "alerting",
        "for": "5m",
        "frequency": "1m",
        "handler": 1,
        "name": "Alert Rule Inside Single Panel",
        "noDataState": "no_data",
        "notifications": []
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
      "description": "",
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "custom": {
            "axisLabel": "",
            "axisPlacement": "auto",
            "barAlignment": 0,
            "drawStyle": "line",
            "fillOpacity": 0,
            "gradientMode": "none",
            "hideFrom": {
              "legend": false,
              "tooltip": false,
              "viz": false
            },
            "lineInterpolation": "linear",
            "lineWidth": 1,
            "pointSize": 5,
            "scaleDistribution": {
              "type": "linear"
            },
            "showPoints": "auto",
            "spanNulls": false,
            "stacking": {
              "group": "A",
              "mode": "none"
            },
            "thresholdsStyle": {
              "mode": "off"
            }
          },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              },
              {
                "color": "red",
                "value": 80
              }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 0
      },
      "id": 6,
      "options": {
        "legend": {
          "calcs": [],
          "displayMode": "list",
          "placement": "bottom"
        },
        "tooltip": {
          "mode": "single",
          "sort": "none"
        }
      },
      "targets": [
        {
          "datasource": "prometheus_datasource_uid",
          "expr": "rate(metric_name[$__interval_rv])",
          "refId": "A"
        }
      ],
      "thresholds": [
        {
          "colorMode": "critical",
          "op": "gt",
          "visible": true
        }
      ],
      "title": "Single Panel",
      "type": "timeseries"
    },
    {
      "aliasColors": {},
      "bars": false,
      "dashLength": 10,
      "dashes": false,
      "fill": 1,
      "fillGradient": 0,
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 0
      },
      "hiddenSeries": false,
      "id": 7,
      "legend": {
        "avg": false,
        "current": false,
        "max": false,
        "min": false,
        "show": true,
        "total": false,
        "values": false
      },
      "lines": true,
      "linewidth": 1,
      "nullPointMode": "null",
      "options": {
        "alertThreshold": true
      },
      "percentage": false,
      "pluginVersion": "8.5.13",
      "pointradius": 2,
      "points": false,
      "renderer": "flot",
      "seriesOverrides": [],
      "spaceLength": 10,
      "stack": false,
      "steppedLine": false,
      "targets": [
        {
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
          "expr": "rate(metric_name[$__interval_rv])",
          "refId": "A"
        }
      ],
      "thresholds": [],
      "timeRegions": [],
      "title": "Plugin Single Panel",
      "tooltip": {
        "shared": true,
        "sort": 0,
        "value_type": "individual"
      },
      "type": "graph",
      "version": 1,
      "xaxis": {
        "mode": "time",
        "show": true,
        "values": []
      },
      "yaxes": [
        {
          "format": "short",
          "logBase": 1,
          "show": true
        },
        {
          "format": "short",
          "logBase": 1,
          "show": true
        }
      ],
      "yaxis": {
        "align": false
      }
    },
    {
      "gridPos": {
        "h": 1,
        "w": 24,
        "x": 0,
        "y": 8
      },
      "id": 4,
      "title": "Row With Panel",
      "type": "row"
    },
    {
      "alert": {
        "alertRuleTags": {},
        "conditions": [
          {
            "evaluator": {
              "params": [
                null
              ],
              "type": "gt"
            },
            "operator": {
              "type": "and"
            },
            "query": {
              "params": [
                "A",
                "5m",
                "now"
              ]
            },
            "reducer": {
              "params": [],
              "type": "avg"
            },
            "type": "query"
          }
        ],
        "executionErrorState": "alerting",
        "for": "5m",
        "frequency": "1m",
        "handler": 1,
        "name": "Panel Inside Row Alert Rule",
        "noDataState": "no_data",
        "notifications": []
      },
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
      "description": "",
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "custom": {
            "axisLabel": "",
            "axisPlacement": "auto",
            "barAlignment": 0,
            "drawStyle": "line",
            "fillOpacity": 0,
            "gradientMode": "none",
            "hideFrom": {
              "legend": false,
              "tooltip": false,
              "viz": false
            },
            "lineInterpolation": "linear",
            "lineWidth": 1,
            "pointSize": 5,
            "scaleDistribution": {
              "type": "linear"
            },
            "showPoints": "auto",
            "spanNulls": false,
            "stacking": {
              "group": "A",
              "mode": "none"
            },
            "thresholdsStyle": {
              "mode": "off"
            }
          },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              },
              {
                "color": "red",
                "value": 80
              }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": {
        "h": 9,
        "w": 12,
        "x": 0,
        "y": 9
      },
      "id": 2,
      "options": {
        "legend": {
          "calcs": [],
          "displayMode": "list",
          "placement": "bottom"
        },
        "tooltip": {
          "mode": "single",
          "sort": "none"
        }
      },
      "targets": [
        {
          "datasource": "prometheus_datasource_uid",
          "expr": "rate(metric_name[$__interval_sx3])",
          "refId": "A"
        }
      ],
      "thresholds": [
        {
          "colorMode": "critical",
          "op": "gt",
          "visible": true
        }
      ],
      "title": "Panel Inside Row",
      "type": "timeseries"
    },
    {
      "cards": {
        "cardHSpacing": 2,
        "cardMinWidth": 5,
        "cardVSpacing": 2
      },
      "color": {
        "cardColor": "#b4ff00",
        "colorScale": "sqrt",
        "colorScheme": "interpolateGnYlRd",
        "defaultColor": "#757575",
        "exponent": 0.5,
        "mode": "spectrum",
        "thresholds": []
      },
      "gridPos": {
        "h": 9,
        "w": 12,
        "x": 12,
        "y": 9
      },
      "hideBranding": false,
      "highlightCards": true,
      "id": 8,
      "legend": {
        "show": true
      },
      "nullPointMode": "as empty",
      "pageSize": 15,
      "seriesFilterIndex": -1,
      "statusmap": {
        "ConfigVersion": "v1"
      },
      "targets": [
        {
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
          "expr": "rate(metric_name[$__interval_sx4])",
          "refId": "A"
        }
      ],
      "title": "Plugin Panel Inside Row",
      "tooltip": {
        "extraInfo": "",
        "freezeOnClick": true,
        "items": [],
        "show": true,
        "showExtraInfo": false,
        "showItems": false
      },
      "type": "flant-statusmap-panel",
      "useMax": true,
      "usingPagination": false,
      "version": 1,
      "xAxis": {
        "show": true
      },
      "yAxis": {
        "maxWidth": -1,
        "minWidth": -1,
        "show": true
      },
      "yAxisSort": "metrics",
      "yLabel": {
        "delimiter": "",
        "labelTemplate": "",
        "usingSplitLabel": false
      }
    }
  ],
  "schemaVersion": 36,
  "style": "dark",
  "tags": [],
  "templating": {
    "list": []
  },
  "time": {
    "from": "now-6h",
    "to": "now"
  },
  "timepicker": {},
  "timezone": "",
  "title": "Test",
  "uid": "test",
  "version": 1,
  "weekStart": ""
}`
	expected := &Messages{[]Message{
		NewError("dashboard.json", "deprecated interval", "Panel Single Panel contains deprecated interval: 'interval_rv', consider using '$__rate_interval'"),
		NewError("dashboard.json", "legacy alert rule", "Panel Single Panel contains legacy alert rule: 'Alert Rule Inside Single Panel', consider using external alertmanager"),
		NewError("dashboard.json", "legacy datasource uid", "Panel Single Panel contains legacy datasource uid: 'prometheus_datasource_uid', consider resaving dashboard using newer version of Grafana"),
		NewError("dashboard.json", "hardcoded datasource uid", "Panel Single Panel contains hardcoded datasource uid: 'prometheus_datasource_uid', consider using grafana variable of type 'Datasource'"),
		NewError("dashboard.json", "deprecated panel type", "Panel Plugin Single Panel is of deprecated type: 'graph', consider using 'timeseries'"),
		NewError("dashboard.json", "deprecated interval", "Panel Plugin Single Panel contains deprecated interval: 'interval_rv', consider using '$__rate_interval'"),
		NewError("dashboard.json", "legacy datasource uid", "Panel Plugin Single Panel contains legacy datasource uid: 'prometheus_datasource_uid', consider resaving dashboard using newer version of Grafana"),
		NewError("dashboard.json", "deprecated interval", "Panel Panel Inside Row contains deprecated interval: 'interval_sx3', consider using '$__rate_interval'"),
		NewError("dashboard.json", "legacy alert rule", "Panel Panel Inside Row contains legacy alert rule: 'Panel Inside Row Alert Rule', consider using external alertmanager"),
		NewError("dashboard.json", "legacy datasource uid", "Panel Panel Inside Row contains legacy datasource uid: 'prometheus_datasource_uid', consider resaving dashboard using newer version of Grafana"),
		NewError("dashboard.json", "hardcoded datasource uid", "Panel Panel Inside Row contains hardcoded datasource uid: 'prometheus_datasource_uid', consider using grafana variable of type 'Datasource'"),
		NewError("dashboard.json", "deprecated panel type", "Panel Plugin Panel Inside Row is of deprecated type: 'flant-statusmap-panel', consider using 'status-history'"),
		NewError("dashboard.json", "deprecated interval", "Panel Plugin Panel Inside Row contains deprecated interval: 'interval_sx4', consider using '$__rate_interval'"),
		NewError("dashboard.json", "legacy datasource uid", "Panel Plugin Panel Inside Row contains legacy datasource uid: 'prometheus_datasource_uid', consider resaving dashboard using newer version of Grafana"),
	}}

	actual := validateGrafanaDashboardFile("dashboard.json", []byte(in))
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expect \n%s\n, got \n%s\n", expected, actual)
	}
}
