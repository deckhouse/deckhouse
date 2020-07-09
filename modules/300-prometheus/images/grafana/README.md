# Grafana

Image with patched Grafana 7. Use v7.0.6

## Patches

### extra variables

Add variables for queries:

```
${__interval_sx3}
${__interval_sx4}
${__interval_rv}
```

### folders

A patch to define folder structure using configuration. See https://github.com/grafana/grafana/pull/23117

This patch can be removed in future updates.

### thin bars

Fast forward feature in Trickster make thin bars in graphs and thin cards in heatmap. It is fixed with filtering single small distance between timestamps for timeStep and card width calculation.
