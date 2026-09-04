---
title: "Frequently Asked Questions"
permalink: en/user/managed-services/faq/
description: "Diagnosing common problems when using managed PostgreSQL in Deckhouse Kubernetes Platform."
---

This page lists common problems when using managed PostgreSQL and how to diagnose them.

## Postgres does not reach the ready state

Check the PostgreSQL instances, events, and PVCs:

```shell
d8 k get pods -n postgres -o wide
d8 k get events -n postgres --sort-by=.lastTimestamp
d8 k get pvc -n postgres
```

If `d8 k apply` rejects the change, check the error message and the constraints of the selected PostgresClass.

## PostgreSQL instances remain in the Pending state

Check the Pod events:

```shell
d8 k get pods -n postgres
d8 k describe pod <POD_NAME> -n postgres
```

Where `<POD_NAME>` is the name of the PostgreSQL instance Pod.

One possible cause is the absence of nodes that match the placement rules of the selected PostgresClass (`nodeSelector`, `nodeAffinity`, or `tolerations`). If no matching nodes are available, contact the cluster administrator or select another available PostgresClass.

## Snapshot issues

Check the VolumeSnapshotClass and VolumeSnapshot:

```shell
d8 k get volumesnapshotclass
d8 k get volumesnapshot -n postgres
```
