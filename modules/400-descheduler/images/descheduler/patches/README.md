# Patches

## 001-filter-pods-in-deckhouse-namespaces.patch

This patch removes pods in `d8-` and `kube-system` namespaces from processing.

## 002-pod-namespace-selector.patch

Adds support of the namespaceSelector in DefaultEvictor plugin.
