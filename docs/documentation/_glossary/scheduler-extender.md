---
title: Scheduler extender
description: An extension of kube-scheduler provided by a Deckhouse Platform module that places pods according to the module's own logic instead of the standard scheduler, for example by local volumes, node selectors of a storage class or network interfaces.
lang: en
---

The [`sds-local-volume`](/modules/sds-local-volume/) module places pods with local volumes by the free space on nodes and the space to reserve for the volumes. Its webhook sets `spec.schedulerName` of such pods to `sds-local-volume`. The [`csi-nfs`](/modules/csi-nfs/) module places pods by the node selectors of NFSStorageClass, and the [`sdn`](/modules/sdn/) module suggests nodes according to the network interface settings of pods using [software-defined networks](glossary.html#sdn). kube-scheduler sends scheduling requests for such pods to the extender's webhook.

For details, see [Storage architecture](architecture/storage/).
