---
title: How can I tell that the update was successful?
subsystems:
  - deckhouse
lang: en
---

If the [`DeckhouseUpdating`](../reference/alerts.html#monitoring-deckhouse-deckhouseupdating) alert is gone, the update has finished.

You can also check the status of DKP releases in the cluster with the following command:

```shell
d8 k get deckhouserelease
```

Example output:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d
v1.46.9    Superseded   11d
v1.47.0    Superseded   4h12m
v1.47.1    Deployed     4h12m
```

The `Deployed` status means the cluster has switched to the corresponding version,
but it doesn’t guarantee that the update has been successful.

To ensure the update completed successfully, check the state of the `deckhouse` Pod with the following command:

```shell
d8 k -n d8-system get pods -l app=deckhouse
```

Example output:

```console
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

- If the Pod is `Running` and shows `1/1` under `READY`, it means the update completed successfully.
- If the Pod is `Running` but shows `0/1` under `READY`, it means the update is still in progress.
  If it stays like this for more than 20–30 minutes, it may indicate a problem with DKP that requires investigation.
- If the Pod is not `Running`, it may indicate a problem with DKP that requires investigation.

#### If something goes wrong

- Check the logs using the following command:

  ```shell
  d8 k -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Collect debug information and contact [DKP technical support](/tech-support/).
