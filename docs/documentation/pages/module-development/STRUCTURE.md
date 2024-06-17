---
title: "Module structure"
permalink: en/module-development/structure/
---

{% raw %}
The source code of the module and its assembly rules must be located in a directory with a certain structure. The best analog is a Helm chart. This page describes the structure of module directories and files.

There is a repository containing the sample [module template](https://github.com/deckhouse/modules-template/). We recommend you start your module development with it.

Below is an example of the directory structure of a module created from a _template_, containing the rules for building and publishing using GitHub Actions:  

```tree
📁 my-module/
├─ 📁 .github/
│  ├─ 📁 workflows/
│  │  ├─ 📝 build_dev.yaml
│  │  ├─ 📝 build_prod.yaml
│  │  ├─ 📝 checks.yaml
│  │  ├─ 📝 deploy_dev.yaml
│  │  └─ 📝 deploy_prod.yaml
├─ 📁 .werf/
│  ├─ 📁 workflows/
│  │  ├─ 📝 bundle.yaml
│  │  ├─ 📝 images.yaml
│  │  ├─ 📝 images-digest.yaml
│  │  ├─ 📝 python-deps.yaml
│  │  └─ 📝 release.yaml
├─ 📁 charts/
│  └─ 📁 helm_lib/
├─ 📁 crds/
│  ├─ 📝 crd1.yaml
│  ├─ 📝 doc-ru-crd1.yaml
│  ├─ 📝 crd2.yaml
│  └─ 📝 doc-ru-crd2.yaml
├─ 📁 docs/
│  ├─ 📝 README.md
│  ├─ 📝 README.ru.md
│  ├─ 📝 EXAMPLES.md
│  ├─ 📝 EXAMPLES.ru.md
│  ├─ 📝 CONFIGURATION.md
│  ├─ 📝 CONFIGURATION.ru.md
│  ├─ 📝 CR.md
│  ├─ 📝 CR.ru.md
│  ├─ 📝 FAQ.md
│  ├─ 📝 FAQ.ru.md
│  ├─ 📝 ADVANCED_USAGE.md
│  └─ 📝 ADVANCED_USAGE.ru.md
├─ 📁 hooks/
│  ├─ 📝 ensure_crds.py
│  ├─ 📝 hook1.py
│  └─ 📝 hook2.py
├─ 📁 images/
│  ├─ 📁 nginx
│  │  └─ 📝 Dockerfile
│  └─ 📁 backend
│     └─ 📝 werf.inc.yaml
├─ 📁 lib/
│  └─ 📁 python/
│     └─ 📝 requirements.txt
├─ 📁 openapi/
│  ├─ 📁 conversions
│  │  ├─ 📁 testdata
│  │  │  ├─ 📝 v1-1.yaml
│  │  │  └─ 📝 v2-1.yaml
│  │  ├─ 📝 conversions_test.go
│  │  └─ 📝 v2.yaml
│  ├─ 📝 config-values.yaml
│  ├─ 📝 doc-ru-config-values.yaml
│  └─ 📝 values.yaml
├─ 📁 templates/
│  ├─ 📝 a.yaml
│  └─ 📝 b.yaml
├─ 📝 .helmignore
├─ 📝 Chart.yaml
├─ 📝 module.yaml
├─ 📝 werf.yaml
└─ 📝 werf-giterminism.yaml
```

## charts

The `/charts` directory contains Helm helper charts used when rendering templates.

Deckhouse Kubernetes Platform (DKP) has its own library for working with templates called [lib-helm](https://github.com/deckhouse/lib-helm). You can read about the library's features [in the lib-helm repository](https://github.com/deckhouse/lib-helm/blob/main/charts/helm_lib/README.md). To add the library to the module, download the [tgz-archive](https://github.com/deckhouse/lib-helm/releases/) with the appropriate release and move it to the `/charts` directory of the module.

## crds

This directory contains [_CustomResourceDefinitions_](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) (CRDs) used by the module components. CRDs are updated every time the module is started, if there are updates.
{% endraw %}

{% alert level="warning" %}
For CRDs from the module's `/crds` directory to be applied in the cluster, the [ensure_crds.py](https://github.com/deckhouse/modules-template/blob/main/hooks/ensure_crds.py) hook must be added from the _module template_. See [`hooks`](#hooks) for more information.
{% endalert %}

{% raw -%}
To render CRDs from the `/crds` directory in the site documentation or documentation module in the cluster, follow these steps:
* create a translation file with a structure identical to the original resource file:
  - in it, keep only the `description` parameters containing the translation text;
  - use the `doc-ru-` prefix in the name: e.g., `/crds/doc-ru-crd.yaml` for `/crds/crd.yaml`.
* create `/docs/CR.md` and `/docs/CR.ru.md` files.

## docs

The `/docs` directory contains the module documentation:

* `README.md` — this file describes what the module is for, what problem it solves and outlines the general architectural principles.

  The ([front matter](https://gohugo.io/content-management/front-matter/)) file metadata as a YAML structure must present in all language versions of the file. You can use the following parameters in the metadata:
  - `title` — **(recommended)** The title of the module description page, for example, "Deckhouse web admin console". It is also used in navigation if `linkTitle` parameter is not specified.
  - `menuTitle` — **(recommended)** The name of the module to show in the menu on the left sidebar of the page, e.g., "Deckhouse Admin". If not set, the name of the directory or repository is used, e.g. `deckhouse-admin`.
  - `linkTitle` — **(optional)** Alternative title for navigation if, for example, the `title` is very long. If not set, the `title` parameter is used.
  - `description` — **(recommended)** A short unique description of the page content (up to 150 characters). It should not repeat the `title'. Goes on with the meaning of the title and reveals it in more detail. It is used during generation of preview links and indexing by search engines, e.g., "The module allows you to fully manage your Kubernetes cluster through a web interface with only mouse skills."
  - `d8Edition` — **(optional)** `ce/be/se/ee`. The minimum edition in which the module is available. The default is `ce`.
  - `moduleStatus` — **(optional)** `experimental`. The status of the module. If a module is labeled as `experimental`, a warning that the code is unstable is displayed on its pages. Also, a special bar in the menu is displayed.

  <div markdown="0">
  <details><summary>Metadata example</summary>
  <pre class="highlight">
  <code>---
  title: "Deckhouse administrator web console"
  menuTitle: "Deckhouse Admin"
  description: "The module allows you to fully manage your Kubernetes cluster through a web interface with only mouse skills."
  ---</code>
  </pre>
  </details>
  </div>

* The `EXAMPLES.md` file contains examples of module configuration with description.
  
  The ([front matter](https://gohugo.io/content-management/front-matter/)) file metadata as a YAML structure must present in all language versions of the file. You can use the following parameters in the metadata:
  - `title` – **(recommended)** The title of the page, e.g., `Examples`. It is also used in navigation if there is no `linkTitle`.
  - `description` – **(recommended)** A short unique description of the page content (up to 150 characters). It should not repeat the `title'. Goes on with the meaning of the title and reveals it in more detail. It is used during generation of preview links and indexing by search engines, e.g., "Examples of storing secrets in a neural network and automatically substituting them into thoughts when communicating."
  - `linkTitle` – **(optional)** Alternative title for navigation if, for example, the `title` is very long. If not set, the `title` parameter is used.  

  <div markdown="0">
  <details><summary>Metadata example</summary>
  <pre class="highlight">
  <code>---
  title: "Examples"
  description: "Examples of storing secrets in a neural network with automatic substitution into thoughts when communicating."
  ---</code>
  </pre>
  </details>
  </div>

* `FAQ.md` – the file contains frequently asked questions related to module operation, e.g., "What scenario should I choose: A or B?".
  
  The ([front matter](https://gohugo.io/content-management/front-matter/)) file metadata as a YAML structure must present in all language versions of the file. You can use the following parameters in the metadata:
  - `title` – **(recommended)** The title of the page.
  - `description` – **(recommended)** A short unique description of the page content (up to 150 characters).
  - `linkTitle` – **(optional)** Alternative title for navigation if, for example, the `title` is very long. If not set, the `title` parameter is used.  

  <div markdown="0">
  <details><summary>Metadata example</summary>
  <pre class="highlight">
  <code>---
  title: "FAQs"
  description: "Frequently asked questions."
  ---</code>
  </pre>
  </details>
  </div>
  
* `ADVANCED_USAGE.md` — this file contains instructions for debugging the module.
  
  The ([front matter](https://gohugo.io/content-management/front-matter/)) file metadata as a YAML structure must present in all language versions of the file. You can use the following parameters in the metadata:
  - `title` – **(recommended)** The title of the page.
  - `description` – **(recommended)** A short unique description of the page content (up to 150 characters).
  - `linkTitle` – **(optional)** Alternative title for navigation if, for example, the `title` is very long. If not set, the `title` parameter is used.  

  <div markdown="0">
  <details><summary>Metadata example</summary>
  <pre class="highlight">
  <code>---
  title: "Module debugging"
  description: "This section covers all the steps for debugging the module."
  ---</code>
  </pre>
  </details>
  </div>
  
* Manually add `CR.md` and `CR.ru.md`, the files for generating resources from the `/crds/` directory.  

  <div markdown="0">
  <details><summary>Metadata example</summary>
  <pre class="highlight">
  <code>---
  title: "Custom resources"
  ---</code>
  </pre>
  </details>
  </div>

* Manually add `CONFIGURATION.md`, the file to create resources from `/openapi/config-values.yaml` and `/openapi/doc-<LANG>-config-values.yaml`.

  <div markdown="0">
  <details><summary>Metadata example</summary>
  <pre class="highlight">
  <code>---
  title: "Module settings"
  ---</code>
  </pre>
  </details>
  </div>
  
All images, PDF files and other media files should be stored in the `/docs` directory or its subdirectories (e.g, `/docs/images/`). All links to files should be relative.

You need a file with the appropriate suffix for each language, e.g. `image1.jpg` and `image1.ru.jpg`. Here's how you can include images in your document:
- `[image1](image1.jpg)` in an English-language document;
- `[image1](image1.ru.jpg)` in a Russian-language document.

## hooks

The `/hooks` directory contains the module's hooks. A hook is an executable file executed in response to an event. Hooks are also used by the module for dynamic interaction with Kubernetes API. For example, they can be used to handle events related to the creation or deletion of objects in a cluster.
{% endraw %}

[Get to know](./#before-you-start) the concept of hooks before you start developing your own hook. You can use the [Python library](https://github.com/deckhouse/lib-python) by the Deckhouse team to speed up the development of hooks.

{% raw %}
Hook requirements:
- The hook must be written in the Python language.
- When run with the `--config` parameter, the hook must output its configuration in YAML format.
- When run without parameters, the hook must perform its intended action.

The hook files must be executable. Add the appropriate permissions using the `chmod +x <path to the hook file>` command.

You can find example hooks in the [module template](https://github.com/deckhouse/modules-template/) repository.

Below is an example of a hook that enables CRDs (from the [/crds](#crds) directory of the module):

```python
import os

import yaml
from deckhouse import hook

# We expect structure with possible subdirectories like this
#
#   my-module/
#       crds/
#           crd1.yaml
#           crd2.yaml
#           subdir/
#               crd3.yaml
#       hooks/
#           ensure_crds.py # this file

config = """
configVersion: v1
onStartup: 5
"""

def main(ctx: hook.Context):
    for crd in iter_manifests(find_crds_root(**file**)):
        ctx.kubernetes.create_or_update(crd)

def iter_manifests(root_path: str):
  if not os.path.exists(root_path):
      return

  for dirpath, dirnames, filenames in os.walk(top=root_path):
      for filename in filenames:
          if not filename.endswith(".yaml"):
              # Wee only seek manifests
              continue
          if filename.startswith("doc-"):
              # Skip dedicated doc yamls, common for Deckhouse internal modules
              continue

      crd_path = os.path.join(dirpath, filename)
      with open(crd_path, "r", encoding="utf-8") as f:
          for manifest in yaml.safe_load_all(f):
              if manifest is None:
                  continue
              yield manifest

  for dirname in dirnames:
      subroot = os.path.join(dirpath, dirname)
      for manifest in iter_manifests(subroot):
          yield manifest

def find_crds_root(hookpath):
    hooks_root = os.path.dirname(hookpath)
    module_root = os.path.dirname(hooks_root)
    crds_root = os.path.join(module_root, "crds")
    return crds_root

if **name** == "**main**":
    hook.run(main, config=config)</code>
```

## images

The `/images` directory contains instructions for building module container images. The first level contains directories for files used to create the container image, the second level contains the building context.

There are two ways to define a container image:

1. [Dockerfile](https://docs.docker.com/engine/reference/builder/) — this file contains commands for building images. To build an application from source code, copy it next to the Dockerfile and include it in the image using the `COPY` command.
2. The `werf.inc.yaml` file, which is the same as the [image definition section in `werf.yaml`](https://werf.io/documentation/v1.2/reference/werf_yaml.html#L33).

The image name matches the directory name for this module, written in _camelCase_ notation starting with a small letter. For example, the directory `/images/echo-server` corresponds to the image name `echoServer`.

The built images have content-based tags that can be used when building other images. To use content-based image tags, [enable the lib-helm](#charts) library. You can also use other features of the [helm_lib library](https://github.com/deckhouse/lib-helm/tree/main/charts/helm_lib) of Deckhouse Kubernetes Platform.

Below is an example of using a content-based image tag in a Helm chart:

```yaml
image: {{ include "helm_lib_module_image" (list . "<image name>") }}
```

## openapi

### conversions

The `/openapi/conversions` directory contains module parameter conversion files and their tests.

Module parameter conversions allow you to convert the OpenAPI specification of module parameters from one version to another. Conversions may be necessary when a parameter is renamed or moved to a different location in a new version of the OpenAPI specification.

Each conversion can only be performed between two consecutive versions (e.g., from the first to the second one). There can be several conversions, and the chain of conversions must cover all versions of the parameter specification with no "gaps".

The conversion file is an arbitrarily named YAML file of the following format:

```yaml
version: N # The version number to convert to. 
conversions: []  # A set of jq expressions to transform data from the previous version.
```

Below is an example of a module parameter conversion file where in version 2, the `.auth.password` parameter has been removed:

```yaml
version: 2
conversions:
  - del(.auth.password) | if .auth == {} then del(.auth) end
```

#### Conversion tests

You can use the `conversion.TestConvert` function to write conversion tests. It receives the following parameters:
- path to the source configuration file (i.e., the version before the conversion);
- path to the resulting configuration file (i.e., the version after the conversion).

An [example](https://github.com/deckhouse/deckhouse/blob/main/modules/300-prometheus/openapi/conversions/conversions_test.go) of a conversion test.

## templates

The `/templates` directory contains [Helm templates](https://helm.sh/docs/chart_template_guide/getting_started/).

* Use the path `.Values.<moduleName>` to access module settings in templates, and `.Values.global` for global settings. The module name is converted to _camelCase_ notation.

* To facilitate working with templates, use [lib-helm](https://github.com/deckhouse/lib-helm), which is a set of extra functions that make it easier to work with global and module values.

* Accesses to the registry from the _ModuleSource_ resource are available at the `.Values.<moduleName>.registry.dockercfg` path.

* To use these functions to pull image pools in controllers, create a secret and add it to the corresponding parameter: `"imagePullSecrets": [{"name":"registry-creds"}]`.

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: registry-creds
  type: kubernetes.io/dockerconfigjson
  data:
    .dockerconfigjson: {{ .Values.<moduleName>.registry.dockercfg }}
  ```

A module can have parameters with which it can alter its behavior. Module parameters and their validation scheme are described in OpenAPI-schemes in `/openapi` directory.

The settings are stored in two files: [`config-values.yaml`](#config-valuesyaml) and [`values.yaml`](#valuesyaml).

You can find an example of an OpenAPI schema in [module template](https://github.com/deckhouse/modules-template/blob/main/openapi/config-values.yaml).

### config-values.yaml

This file is required to validate the module parameters that the user can configure via [_ModuleConfig_](deckhouse.ru.md#the-moduleconfig-resource).

To render the schema in the documentation on the site or in the documentation module in the cluster, create:
- the `doc-ru-config-values.yaml` file with a structure similar to that of the `config-values.yaml` file. Keep only the translated description parameters in the `doc-ru-config-values.yaml` file;
- the `/docs/CONFIGURATION.md` and `/docs/CONFIGURATION.ru.md` files to enable rendering of data from the `/openapi/config-values.yaml` and `/openapi/doc-ru-config-values.yaml` files.

An example of a `/openapi/config-values.yaml` schema with a single configurable `nodeSelector` parameter:

```yaml
type: object
properties:
  nodeSelector:
    type: object
    additionalProperties:
      type: string
    description: |
      The same as the Pods' `spec.nodeSelector` parameter in Kubernetes.

      If the parameter is omitted or `false`, `nodeSelector` will be determined
      [automatically](https://deckhouse.io/documentation/v1/#advanced-scheduling).</code>
```

An example of the `/openapi/doc-ru-config-values.yaml` file for the Russian translation of the schema:

```yaml
properties:
  nodeSelector:
    description: |
      Russian description. Markdown markup.</code>
```

### values.yaml

This file is required for validating the source data when rendering templates without using extra Helm chart functions.
Its closest analogs are Helm's [schema files](https://helm.sh/docs/topics/charts/#schema-files).

You can automatically add parameter validation from `config-values.yaml` to `values.yaml`. In this case, the basic `values.yaml` looks as follows:

```yaml
x-extend:
  schema: config-values.yaml
type: object
properties:
  internal:
    type: object
    default: {}</code>
```

## .helmignore

`.helmignore` allows you to exclude files from the Helm release. In case of DKP modules, directories `/crds`, `/images`, `/hooks`, `/openapi` must be added to `.helmignore` to avoid exceeding 1 Mb limit of Helm release size.

## Chart.yaml

This is a mandatory file for a chart, similar to [`Chart.yaml`](https://helm.sh/docs/topics/charts/#the-chartyaml-file) in Helm. It must contain at least a `name` parameter with the module name and a `version` parameter with the version.

An example:

```yaml
name: echoserver
version: 0.0.1
dependencies:
- name: deckhouse_lib_helm
  version: 1.5.0
  repository: https://deckhouse.github.io/lib-helm
```

## module.yaml

This file stores the following module settings:

- `tags: string` — the additional module tags, which are converted to module labels: `module.deckhouse.io/$tag=""`.
- `weight: integer` — the module weight. The default weight is 900, you can set your own weight between 900 and 999.
- `stage: string` — [module lifecycle stage](versioning/#module-lifecycle). Can be `Sandbox`, `Incubating`, `Graduated`, or `Deprecated`.
- `description: string` — the module description.

An example:

```yaml
tags: ["test", "myTag"]
weight: 960
stage: "Sandbox"
description: "my awesome module"
```

Applying this file will create a module (`deckhouse.io/v1alpha/Module`) with the labels `module.deckhouse.io/test=""` and `module.deckhouse.io/myTag=""`, weight `960`, and the description `my awesome module`.

This way you can control the module sequence as well as specify additional meta-information for the modules.
{% endraw %}
