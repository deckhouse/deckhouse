---
title: "Module development and debugging"
permalink: en/module-development/development/
---

{% raw %}

When developing modules, you may want to pull and deploy a module bypassing the release channels. The [ModulePullOverride](../../cr.html#modulepulloverride) resource is used for this purpose.

An example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModulePullOverride
metadata:
  name: <module-name>
spec:
  imageTag: <tag of the module image>
  scanInterval: <image digest check interval. Default: 15s>
  source: <ModuleSource ref>
```

Requirements for the resource parameters:
* The **metadata.name** module name must match the module name in the *ModuleSource* (the `.status.modules.[].name` parameter).

* The **spec.imageTag** container image tag can be anything, e.g., ~pr333~, ~my-branch~.

* The *ModuleSource* **spec.source** parameter provides the data for registry authorization.

The **spec.scanInterval** time interval (optional) defines the interval for scanning images in the registry. The default interval is 15 seconds.

You can specify a longer interval to force a refresh, and use the `renew=“”` annotation.

Below is an example of the command:

```sh
kubectl annotate mop <name> renew=""
```

## How it works

When developing this resource, the specified module will not consider *ModuleUpdatePolicy*, nor will it load or create *ModuleRelease* objects.

Instead, the module will be pulled every time the `imageDigest` parameter is changed and it will be applied in the cluster.
At the same time, that module will get the `overridden: true` attribute in the status of the [ModuleSource](../../cr.html#modulesource) resource, indicating that the [ModulePullOverride](../../cr.html#modulepulloverride) resource is being used.

The module will keep running after *ModulePullOverride* is removed. However, if the [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy) policy is applied to the module, new releases (if available) will be pulled to replace the current "developer version".

### An example

1. Suppose there are two modules, `echo` and `hello-world`, defined in [ModuleSource](../../cr.html#modulesource). The update policy is set for them, and they are pulled in and installed in DKP:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: test
   spec:
     registry:
       ca: ""
       dockerCfg: someBase64String==
       repo: registry.example.com/deckhouse/modules
       scheme: HTTPS
   status:
     modules:
     - name: echo
       policy: test-alpha
     - name: hello-world
       policy: test-alpha
     modulesCount: 2
   ```

1. Create a [ModulePullOverride](../../cr.html#modulepulloverride) resource for the `echo` module:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     source: test
   ```

   This resource will be validating the `registry.example.com/deckhouse/modules/echo:main-patch-03354` image tag (`ms:spec.registry.repo/mpo:metadata.name:mpo:spec.imageTag`).

1. The status of this resource will change with each update:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     scanInterval: 15s
     source: test
   status:
     imageDigest: sha256:ed958cc2156e3cc363f1932ca6ca2c7f8ae1b09ffc1ce1eb4f12478aed1befbc
     message: ""
     updatedAt: "2023-12-07T08:41:21Z"
   ```

   where:
   - **imageDigest** is the unique identifier of the container image that was pulled.
   - **lastUpdated** is the time when the image was last pulled.

1. In this case, *ModuleSource* would look as follows:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: test
   spec:
     registry:
       ca: ""
       dockerCfg: someBase64String==
       repo: registry.example.com/deckhouse/modules
       scheme: HTTPS
   status:
     modules:
     - name: echo
       overridden: true
     - name: hello-world
       policy: test-alpha
     modulesCount: 2
   ```

{% endraw %}

## Module artifacts in the container registry

After a module has been built, its artifacts must be pushed to the container registry at a path that is the *source* path for pulling and running modules in DKP. The path where module artifacts are pushed to the registry is specified in the [ModuleSource](../../cr.html#modulesource) resource.

Below is an example of the container image hierarchy after pushing the `module-1` and `modules-2` module artifacts into the registry:

```tree
registry.example.io
📁 modules-source
├─ 📁 module-1
│  ├─ 📦 v1.23.1
│  ├─ 📦 d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
│  ├─ 📦 e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
│  ├─ 📦 v1.23.2
│  └─ 📁 release
│     ├─ 📝 v1.23.1
│     ├─ 📝 v1.23.2
│     ├─ 📝 alpha
│     └─ 📝 beta
└─ 📁 module-2
   ├─ 📦 v0.30.147
   ├─ 📦 d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
   ├─ 📦 e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
   ├─ 📦 v0.31.1
   └─ 📁 release
      ├─ 📝 v0.30.147
      ├─ 📝 v0.31.1
      ├─ 📝 alpha
      └─ 📝 beta
```

{% alert level="warning" %}
The container registry must support a nested repository structure. See [the requirements section](module-development/#requirements) for more details.  
{% endalert %}

Below is a list of commands for working with the module source. The examples use the [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane#crane) tool. Follow the [instructions](https://github.com/google/go-containerregistry/tree/main/cmd/crane#installation) to install it. For macOS, use `brew`.

### Print the list of modules in the module source

```shell
crane ls <REGISTRY_URL>/<MODULE_SOURCE>
```

An example:

```shell
$ crane ls registry.example.io/modules-source
module-1
module-2
```

### Print the list of module images

```shell
crane ls <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>
```

An example:

```shell
$ crane ls registry.example.io/modules-source/module-1
v1.23.1
d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
v1.23.2
```

In the example above, there are two module images and two application container images in `module-1`.

### Print the list of files in the module image

```shell
crane export <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>:<MODULE_TAG> - | tar -tf -
```

An example:

```shell
crane export registry.example.io/modules-source/module-1:v1.23.1 - | tar -tf -
```

The output will be quite large.

### Print the list of images of the module's application containers @TODO <-- переформулировать

```shell
crane export <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>:<MODULE_TAG> - | tar -Oxf - images_digests.json
```

An example:

```shell
$ crane export registry.example.io/modules-source/module-1:v1.23.1 -  | tar -Oxf - images_digests.json
{
  "backend": "sha256:fcb04a7fed2c2f8def941e34c0094f4f6973ea6012ccfe2deadb9a1032c1e4fb",
  "frontend": "sha256:f31f4b7da5faa5e320d3aad809563c6f5fcaa97b571fffa5c9cab103327cc0e8"
}
```

### Print the list of releases

```shell
crane ls <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>/release
```

An example:

```shell
$ crane ls <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>/release
v1.23.1
v1.23.2
alpha
beta
```

In the example above, there are two releases in the container registry; two release channels, `alpha` and `beta`, are also used:

### Print the version in use on the `alpha` release channel

```shell
crane export <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>/release:alpha - | tar -Oxf - version.json
```

An example:

```shell
$ crane export registry.example.io/modules-source/module-1/release:alpha - | tar -Oxf - version.json
{"version":"v1.23.2"}
```
