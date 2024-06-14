---
title: "How to start module in the DKP claster?"
permalink: en/module-development/run/
---

This section covers the process of running a configured module in a cluster managed by Deckhouse Kubernetes Platform (DKP).

Follow these steps to run the module in a cluster:

- [Define ModuleSource](#module-source) (the [ModuleSource](../../cr.html#modulesource) resource).
- _(optional)_ Define the [module update policy](#module-update-policy) (the [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy) resource).
- [Enable the module in the cluster](#enabling-the-module) (the [ModuleConfig](../../cr.html#moduleconfig) resource).
  
## Module source

Create a [ModuleSource](../../cr.html#modulesource) resource to set the source to fetch module information from. This resource will contain the address of the container registry to pull modules from, authentication parameters, and other settings.

An example of a `ModuleSource` resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: example
spec:
  registry:
    repo: registry.example.com/deckhouse/modules
    dockerCfg: <base64 encoded credentials>
```

After the `ModuleSource` resource is created, DKP will start to perform periodic (every three minutes) data synchronization with the module source (fetching information about the modules available in the source).

Use the following command to check the synchronization status:

```shell
kubectl get ms
```

If the synchronization is successful, you will see output similar to the one below:

```console
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s
```

If there are synchronization errors, the `MSG` column will contain a general description of the error, e.g.:

```console
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Some errors occurred. Inspect status for details
```

Detailed error information can be found in the `status.moduleErrors` field of the _ModuleSource_ resource.

For example, here's how you can get detailed error description from the `example` module source:

```console
$ kubectl  get ms example -o jsonpath='{range .status.moduleErrors[*]}{.name}{" module error:\n\t"}{.error}{"\n"}{end}'
module-1 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-1/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
module-2 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-2/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
```

If synchronization is successful, the `.status.modules` field of the _ModuleSource_ resource will contain a list of modules ready to be enabled in the cluster.

Here is an example of how you can get a list of modules available from the `example` module source:

```console
$ kubectl get ms example -o jsonpath='{.status.modules[*].name}'
module-1 module-2
```

The complete list of modules available from all module sources created in the cluster can be retrieved using the following command:

```shell
kubectl get ms  -o jsonpath='{.items[*].status.modules[*].name}'
```

After creating the `ModuleSource` resource and successful synchronization, _module releases_, i. e., [ModuleRelease](cr.html#modulerelease) resources will begin to be created in the cluster (DKP creates them automatically, you don't need to do it manually). Use the following command to print the list of releases:

```shell
kubectl get mr
```

An example of retrieving the list of module releases:

```console
$ kubectl get mr
NAME                       PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
module-one-v1.21.3         Superseded   deckhouse       33h              
module-one-v1.22.0         Deployed     deckhouse       33h              
module-two-v1.2.0          Superseded   deckhouse       48d              
module-two-v1.2.1          Superseded   deckhouse       48d              
module-two-v1.2.3          Deployed     deckhouse       48d              
module-two-v1.2.4          Superseded   deckhouse       44d              
module-two-v1.2.5          Pending      deckhouse       44d              Waiting for manual approval

```

If there is a module release in `Deployed` status, this module can be [enabled](#enable-module) in the cluster. If a module release is in `Superseded` status, it means that the module release is out of date, and there is a newer release to replace it.

{% alert level="warning" %}
If a module release is Pending, it means that manual confirmation is required to install it (see [module update policy](#module-update-policy) below). You can confirm the module release using the following command (specify the _moduleRelease_ name):

```shell
kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
```

{% endalert %}

### Switching the module to a different module source

Follow these steps to deploy a module from a different module source:
1. Find out what [update policy](#module-update-policy) is used for the module:

   ```shell
   kubectl get mr
   ```

   Look up the `UPDATE POLICY` for the module releases.

2. Before dropping this update policy, make sure there are no releases awaiting to be deployed (in Pending state) that fall under the policy being dropped or modified (or the _labelSelector_ used by the policy no longer matches your module):

   ```shell
   kubectl delete mup <POLICY_NAME>
   ```

3. Create a new [ModuleSource](#module-source) resource.

4. Create a new [ModuleUpdatePolicy](#module-update-policy) resource with the correct labels (source) for the new _ModuleSource_.

5. Confirm that new _ModuleReleases_ for a module are created from a new _ModuleSource_ according to the update policy.

   ```shell
   kubectl get mr
   ```

## Module update policy

The module update policy refers to the rules that DKP uses to update modules in the cluster. It is set by the [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy) resource with the following settings:
- module update mode (automatic, manual, updates are disabled);
- the release channel to use for updates;
- time windows for automatic updates during which the module update is permitted.

You do not have to create the `ModuleUpdatePolicy` resource. If the update policy for a module is not defined (there is no corresponding `ModuleUpdatePolicy` resource), the update settings match the update settings of DKP (the [update](../../modules/002-deckhouse/configuration.html#parameters-update) parameter of the `deckhouse` module).

{% alert level="info" %}
To avoid downloading modules defined in `ModuleUpdatePolicy`, set the [spec.update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) parameter to `Ignore`.
{% endalert %}

{% alert level="warning" %}
If a module is subject to more than one update policy (condition in the `labelSelector` parameter), the modules will not be updated until the module becomes subject to a single update policy.
{% endalert %}

The following is an example of the `ModuleUpdatePolicy` resource that defines the update policy for the `module-1` module of the `example` module source (the `example` ModuleSource). The update policy enables automatic module updates on Mondays and Wednesdays between 13:30 and 14:00 UTC:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleUpdatePolicy
metadata:
  name: example-update-policy
spec:
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: example
        module: module-1
  releaseChannel: Alpha
  update:
    mode: Auto
    windows:
    - days:
      - "Mon"
      - "Wed"
      from: "13:30"
      to: "14:00"
```

### moduleReleaseSelector — usage examples

- Apply the policy to all _ModuleSource_ `deckhouse` modules:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: deckhouse
  ```

- Apply the policy to the `deckhouse-admin` module independently of _ModuleSource_:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        module: deckhouse-admin
  ```

- Apply the policy to the `deckhouse-admin` module from the `deckhouse` _ModuleSource_:
  
  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        module: deckhouse-admin
        source: deckhouse
  ```

- Apply the policy only to the `deckhouse-admin` and `secrets-store-integration` modules in the `deckhouse` _ModuleSource_:
  
  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchExpressions:
      - key: module
        operator: In
        values:
        - deckhouse-admin
        - secrets-store-integration
      matchLabels:
        source: deckhouse
  ```

- Apply the policy to all `deckhouse` _ModuleSource_ modules except for `deckhouse-admin`:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchExpressions:
      - key: module
        operator: NotIn
        values:
        - deckhouse-admin
      matchLabels:
        source: deckhouse
  ```

## Enabling the module

Прежде чем включить модуль, проверьте что он доступен для включения. Выполните следующую команду, чтобы вывести список всех доступных модулей DKP:

```shell
kubectl get modules
```

Модуль должен быть в списке.

Пример вывода:

```console
$ kubectl get modules
NAME                                  WEIGHT   STATE      SOURCE
...
module-test                           900      Disabled   example
...
```

Вывод показывает, что модуль `module-test` доступен для включения.

Если модуля нет в списке, то проверьте что определен [источник модулей](#источник-модулей) и модуль есть в списке в источнике модулей. Также проверьте [политику обновления](#политика-обновления-модуля) модуля (если она определена). Если политика обновления модуля не определена, то она соответствует политике обновления DKP (параметр [releaseChannel](../../modules/002-deckhouse/configuration.html#parameters-releasechannel) и секция [update](../../modules/002-deckhouse/configuration.html#parameters-update) параметров модуля `deckhouse`).

Включить модуль можно аналогично встроенному модулю DKP любым из следующих способов:
- Выполнить следующую команду (укажите имя модуля):

  ```shell
  kubectl -ti -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable <MODULE_NAME>
  ```

- Создать ресурс `ModuleConfig` с параметром `enabled: true` и настройками модуля.

  Пример [ModuleConfig](../../cr.html#moduleconfig), для включения и настройки модуля `module-1` в кластере:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: module-1
  spec:
    enabled: true
    settings:
      parameter: value
    version: 1
  ```

### Troubleshooting

Если при включении модуля в кластере возникли ошибки, то получить информацию о них можно следующими способами:
- Посмотреть журнал DKP:

  ```shell
  kubectl -n d8-system logs -l app=deckhouse
  ```

- Посмотреть ресурс `ModuleConfig` модуля:

  Пример вывода информации об ошибке модуля `module-1`:

  ```shell
  $ kubectl get moduleconfig module-1
  NAME        ENABLED   VERSION   AGE   MESSAGE
  module-1    true                7s    Ignored: unknown module name
  ```

По аналогии [с _DeckhouseRelease_](../../cr.html#deckhouserelease) (ресурсом релиза DKP) у модулей есть аналогичный ресурс — [_ModuleRelease_](../../cr.html#modulerelease). DKP создает ресурсы _ModuleRelease_ исходя из того, что хранится в container registry. При поиске проблем с модулем проверьте также доступные в кластере релизы модуля:

```shell
kubectl get mr
```

Пример вывода:

```shell
$ kubectl get mr
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for manual approval
```

В примере вывода показан _ModuleRelease_, когда режим обновления (параметр [update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) ресурса _ModuleUpdatePolicy_ установлен в `Manual`. В этом случае необходимо вручную подтвердить установку новой версии модуля, установив на релиз аннотацию `modules.deckhouse.io/approved="true"`:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```
