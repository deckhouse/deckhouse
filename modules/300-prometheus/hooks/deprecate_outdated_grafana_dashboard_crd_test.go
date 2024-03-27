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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Prometheus hooks :: deprecate outdated grafana dashboard crd ::", func() {
	f := HookExecutionConfigInit(`{"prometheus":{"internal":{"grafana":{}}}}`, ``)
	f.RegisterCRD("deckhouse.io", "v1", "GrafanaDashboardDefinition", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		Context("After adding outdated GrafanaDashboardDefinition", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: Test
spec:
  definition: '{
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
  "id": 80,
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
      "id": 13,
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
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
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
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 0
      },
      "hideBranding": false,
      "highlightCards": true,
      "id": 15,
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
          "expr": "rate(metric_name[$__interval_rv])",
          "refId": "A"
        }
      ],
      "title": "Plugin Single Panel",
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
        "y": 9
      },
      "id": 11,
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
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
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
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 9
      },
      "hideBranding": false,
      "highlightCards": true,
      "id": 9,
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
    "from": "now-1h",
    "to": "now"
  },
  "timepicker": {},
  "timezone": "",
  "title": "Test",
  "uid": "test",
  "version": 1,
  "weekStart": ""
  }'
`, 1))
				f.RunHook()
			})

			It("Should start exposing metrics about deprecation", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(8))
				Expect(m[0].Name).To(Equal("d8_grafana_dashboards_deprecated_interval"))
				Expect(m[0].Labels).To(Equal(map[string]string{
					"dashboard": "test",
					"panel":     "single_panel",
					"interval":  "interval_rv",
				}))
				Expect(m[1].Name).To(Equal("d8_grafana_dashboards_deprecated_alert_rule"))
				Expect(m[1].Labels).To(Equal(map[string]string{
					"dashboard":  "test",
					"panel":      "single_panel",
					"alert_rule": "alert_rule_inside_single_panel",
				}))
				Expect(m[2].Name).To(Equal("d8_grafana_dashboards_deprecated_interval"))
				Expect(m[2].Labels).To(Equal(map[string]string{
					"dashboard": "test",
					"panel":     "plugin_single_panel",
					"interval":  "interval_rv",
				}))
				Expect(m[3].Name).To(Equal("d8_grafana_dashboards_deprecated_plugin"))
				Expect(m[3].Labels).To(Equal(map[string]string{
					"dashboard": "test",
					"panel":     "plugin_single_panel",
					"plugin":    "flant-statusmap-panel",
				}))
				Expect(m[4].Name).To(Equal("d8_grafana_dashboards_deprecated_interval"))
				Expect(m[4].Labels).To(Equal(map[string]string{
					"dashboard": "test",
					"panel":     "panel_inside_row",
					"interval":  "interval_sx3",
				}))
				Expect(m[5].Name).To(Equal("d8_grafana_dashboards_deprecated_alert_rule"))
				Expect(m[5].Labels).To(Equal(map[string]string{
					"dashboard":  "test",
					"panel":      "panel_inside_row",
					"alert_rule": "panel_inside_row_alert_rule",
				}))
				Expect(m[6].Name).To(Equal("d8_grafana_dashboards_deprecated_interval"))
				Expect(m[6].Labels).To(Equal(map[string]string{
					"dashboard": "test",
					"panel":     "plugin_panel_inside_row",
					"interval":  "interval_sx4",
				}))
				Expect(m[7].Name).To(Equal("d8_grafana_dashboards_deprecated_plugin"))
				Expect(m[7].Labels).To(Equal(map[string]string{
					"dashboard": "test",
					"panel":     "plugin_panel_inside_row",
					"plugin":    "flant-statusmap-panel",
				}))
			})

			Context("And after deleting GrafanaDashboardDefinition", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 1))
					f.RunHook()
				})

				It("Should stop exposing deprecation metrics", func() {
					Expect(f).To(ExecuteSuccessfully())
					m := f.MetricsCollector.CollectedMetrics()
					Expect(m).To(HaveLen(0))
				})
			})

			Context("And after updating GrafanaDashboardDefinition", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: Test
spec:
  folder: test
  definition: '{
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
  "id": 80,
  "links": [],
  "liveNow": false,
  "panels": [
    {
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
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
      "id": 13,
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
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
          "expr": "rate(metric_name[$__rate_interval])",
          "refId": "A"
        }
      ],
      "thresholds": [],
      "title": "Single Panel",
      "type": "timeseries"
    },
    {
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "thresholds"
          },
          "custom": {
            "fillOpacity": 70,
            "lineWidth": 1
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
        "x": 12,
        "y": 0
      },
      "id": 15,
      "options": {
        "colWidth": 0.9,
        "legend": {
          "displayMode": "list",
          "placement": "bottom"
        },
        "rowHeight": 0.9,
        "showValue": "auto",
        "tooltip": {
          "mode": "single",
          "sort": "none"
        }
      },
      "targets": [
        {
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
          "expr": "rate(metric_name[$__rate_interval])",
          "refId": "A"
        }
      ],
      "title": "Plugin Single Panel",
      "type": "status-history"
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
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
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
        "y": 9
      },
      "id": 11,
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
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
          "expr": "rate(metric_name[$__rate_interval])",
          "refId": "A"
        }
      ],
      "thresholds": [],
      "title": "Panel Inside Row",
      "type": "timeseries"
    },
    {
      "datasource": {
        "type": "prometheus",
        "uid": "prometheus_datasource_uid"
      },
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "thresholds"
          },
          "custom": {
            "fillOpacity": 70,
            "lineWidth": 1
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
        "x": 12,
        "y": 9
      },
      "id": 9,
      "options": {
        "colWidth": 0.9,
        "legend": {
          "displayMode": "list",
          "placement": "bottom"
        },
        "rowHeight": 0.9,
        "showValue": "auto",
        "tooltip": {
          "mode": "single",
          "sort": "none"
        }
      },
      "targets": [
        {
          "datasource": {
            "type": "prometheus",
            "uid": "prometheus_datasource_uid"
          },
          "expr": "rate(metric_name[$__rate_interval])",
          "refId": "A"
        }
      ],
      "title": "Plugin Panel Inside Row",
      "type": "status-history"
    }
  ],
  "schemaVersion": 36,
  "style": "dark",
  "tags": [],
  "templating": {
    "list": []
  },
  "time": {
    "from": "now-1h",
    "to": "now"
  },
  "timepicker": {},
  "timezone": "",
  "title": "Test",
  "uid": "test",
  "version": 2,
  "weekStart": ""
  }'
`, 1))
					f.RunHook()
				})

				It("Should stop exposing deprecation metrics", func() {
					Expect(f).To(ExecuteSuccessfully())
					m := f.MetricsCollector.CollectedMetrics()
					Expect(m).To(HaveLen(0))
				})
			})
		})
	})

})
