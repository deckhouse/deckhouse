## Patches

### Extra variables for Prometheus datasource

Add variables for queries:

```
${__interval_sx3}
${__interval_sx4}
${__interval_rv}
```

### Thin bars on heatmap panels

Fast forward feature in Trickster make thin bars in graphs and thin cards in heatmap. It is fixed with filtering single small distance between timestamps for timeStep and card width calculation.

### Useful version

Patch build.go to display useful information in Help popup menu.

### Copy bundled plugins

Copy bundled plugins from $BUNDLED_PLUGINS_PATH if $GF_INSTALL_PLUGINS is set or BUNDLED_PLUGINS_PATH is not equal to GF_PATHS_PLUGINS.
