# Patches

## 001-l2-speaker-preffered-node.patch

Added preferred L2 speaker node feature

Upstream <https://github.com/metallb/metallb/pull/2246/>

## 002-l2lbservice-custom-resource.patch

Added a custom resource L2LBService to replace the original Service.

The controllers logic is rewritten to watch this new private resource.
