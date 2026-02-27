---
title: What should I do if the module image did not download and the module did not reinstall?
lang: en
---

In some cases, there may be a problem with automatically downloading the image and reinstalling the module. These cases include:

- Damage to the file system or other problems that have rendered the module image invalid.
- Switching to a different registry.
- Switching from one DKP edition to another.

In this case, the module may be in the `Ready` state. The error occurs in the module's pods. To find the problematic pod, use the command:

```shell
d8 k -n d8-<module-name> get pods
```

A problematic pod will have a status other than `Running`.

To view information about a pod, use the command:

```shell
d8 k -n d8-<module-name> describe pod <pod-name>
```

Example of an error message in the pod when there is a problem with downloading the image and reinstalling the module:

```console
Failed to pull image "registry.deckhouse.ru/deckhouse/ce/modules/console@sha256:a12b4f8de1d997005155d0ba0a7c968a015dd8d18bb5d54645ddb040ddab1ef4": rpc error: code = NotFound desc = failed to pull and unpack image "registry.deckhouse.ru/deckhouse/ce/modules/console@sha256:a12b4f8de1d997005155d0ba0a7c968a015dd8d18bb5d54645ddb040ddab1ef4": failed to resolve reference ...
```

To download the image and reinstall the module that caused the problem:

1. Get a list of module releases:

   ```shell
   d8 k get mr -l module=my-module
   ```

   Output example:

   ```console
   NAME               PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
   my-module-v3.7.4   Superseded                   5d23h
   my-module-v3.7.5   Deployed                     5d23h
   ```

   Find the module release deployed in the cluster in the list (it should have the status `Deployed`).

1. Add the annotation `modules.deckhouse.io/reinstall=true` to the expanded release:

   ```shell
   d8 k annotate mr my-module-v3.7.5 modules.deckhouse.io/reinstall=true
   ```

After adding the annotation, the module image is re-downloaded from the registry, the module is validated with the current settings from `ModuleConfig`, and installed in the cluster. After successful reinstallation, the annotation is automatically removed from `ModuleRelease`.

To verify that the module has been successfully reinstalled and all module pods is working, use the command:

```shell
d8 k -n d8-<module-name> get pods
```

All pods in the module must have the status `Running`. Example:

```console
NAME                                READY   STATUS    RESTARTS   AGE
backend-567d6c6cdc-g5qgt            1/1     Running   0          2d2h
frontend-7c8b567759-h8jdf           1/1     Running   0          2d2h
observability-gw-86cf75f5d6-7xljh   1/1     Running   0          2d2h
```
