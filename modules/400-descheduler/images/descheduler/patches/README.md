# Patches

## 001-pod-namespace-selector 

Adds support of the namespaceSelector in DefaultEvictor plugin.

## 002-filter-pods-in-deckhouse-namespaces

This patch removes pods in `d8-` and `kube-system` namespaces from processing.
