# Patches

## 000-update-go-libraries.patch

Update golang libraries and dependencies.

## 001-l2lb-speaker-preffered-node.patch

Add preferred L2 speaker node feature.

Upstream <https://github.com/metallb/metallb/pull/2246/>

## 002-l2lb-service-custom-resource.patch

Add a custom resource L2LBService to replace the original Service.

The controllers logic is rewritten to watch this new private resource.

## 003-l2lb-annotation-for-pools.patch

Add the ability to use only IPAddressPool with the annotation 'heritage=deckhouse'.

## 004-l2lb-disable-bgp.patch

Prohibit initializing the BGP controller.
