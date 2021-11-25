# Patches

## Dont announce from annotated nodes

This patch stops BGP announces from nodes annotated by `node.kubernetes.io/exclude-from-external-load-balancers` annotation, which
set by MCM to downscale nodes.

