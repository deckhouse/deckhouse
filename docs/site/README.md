# Deckhouse documentation website

> This README is a work in progress. Some information may be incomplete or outdated.

This document describes the architecture of Deckhouse documentation and explains how to run the documentation website locally.

## Running the documentation site locally

### Requirements

- Clone this repository.
- Ensure that port `80` is available for binding.
- Install [werf](https://werf.io/getting_started/).
  When installing, set the release channel to `alpha`.

### Running the documentation site using separate services

#### Starting the documentation site

To start the documentation site, open two separate terminal windows or tabs and follow these steps:

1. In the first terminal, run:

   ```shell
   cd docs/documentation
   make up
   ```

1. In the second terminal, run:

   ```shell
   cd docs/site
   make up
   ```

1. Open the documentation site in your browser at <http://localhost>.

#### Stopping the documentation site

To stop the documentation site, cancel the running processes and run the following command in both terminals:

```shell
make down
```

### Running the documentation site using a single command

#### Starting the documentation site

To start the documentation site, open a terminal and follow these steps:

1. Run the following command:

   ```shell
   make docs
   ```

1. Open the documentation site in your browser at <http://localhost>.

If you cloned the Deckhouse repository and made uncommitted changes, trying to run the documentation site will result in an error from werf stating that the changes must be committed first.

To bypass that restriction and run the documentation site with uncommitted changes, run the following command:

```shell
make docs-dev
```

#### Stopping the documentation site

To stop the documentation site, cancel the running process and run the following command in the terminal:

```shell
make docs-down
```

## Debugging

The [Delve](https://github.com/go-delve/delve) debugger is used for debugging the documentation site's backend.

Files available for debugging:

- `docs/site/werf-debug.yaml`: Used for compiling the backend.
- `docs/site/docker-compose-debug.yml`: Used for running the backend.

To run the debugger:

1. Navigate to the `docs/site` directory and run the following command:

   ```shell
   werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'
   ```

   Alternatively, run `docs/site/backend/debug.sh`.

1. Once the process is running, connect to `localhost:2345`.

## Working with spellchecker

> Run the following commands from the root of the repository.

Spellchecking commands:

- `make docs-spellcheck`: Check all documentation in the repository for spelling errors.
- `file=<PATH_TO_FILE> make docs-spellcheck`: Check a specific file for spelling errors.

  Example:

  ```shell
  file=ee/se-plus/modules/cloud-provider-vsphere/docs/CONFIGURATION_RU.md make docs-spellcheck`
  ```

- `make docs-spellcheck-generate-dictionary`: Generate a word dictionary. Run it after adding new words to the `tools/docs/spelling/wordlist` file.
- `make docs-spellcheck-get-typos-list`: Get a sorted list of typos from the documentation.
- `make lint-doc-spellcheck-pr`: Used in CI to check the spelling of documentation in a PR.

## Architecture

> ![NOTE] Architecture has been updated. This section is a work in progress. Some information may be incomplete or outdated.

The Deckhouse website consists of the following parts:

- **Main website**. Includes all sections except those specifically described below.
- **Non-versioned documentation**. Includes the following sections:

  - `/products/kubernetes-platform/gs/`
  - `/products/kubernetes-platform/guides/`
  - `/assets/`
  - `/images/`
  - `/presentations/`
  - `/products/virtualization-platform/documentation/`
  - `/products/virtualization-platform/gs/`
  - `/products/virtualization-platform/guides/`
  - `/products/virtualization-platform/reference/`
  
  The content is generated using Jekyll from the `docs/site` directory.
  
- **Versioned documentation**. Includes the following sections:
  
  - `/products/kubernetes-platform/documentation/`

  The content is generated using Jekyll from the `docs/documentation` directory.
  Contains documentation for Deckhouse Kubernetes Platform (DKP) and built-in modules.

- **Documentation for DKP modules**. Includes the following sections:

  - `/products/kubernetes-platform/modules/`

  The content is generated using Hugo:
  
  - Project files for Hugo are located in the `docs/site/backends/docs-builder-template` directory.
  - The documentation builder (written in Go) is located in the `docs/site/backends/docs-builder` directory.

### Structure of the Jekyll-based projects

The project uses [werf](werf.io) to build and deploy documentation.

Things to note:

- The `_tool` directory contains scripts used for building the documentation.
- The `_assets` directory stores assets (styles and scripts), which are used by Jekyll Asset Pipeline plugin.
  Assets are compiled and minified into the `/assets` directory (absolute path) and include a digest in their path.
  If you don't need a digest in the path, use the `/css` or `/js` directory instead.
  In this case, assets will be processed by Jekyll as usual.
  
  Example of including JavaScript assets:

  ```liquid
  <script type="text/javascript" src="
  {%- javascript_asset_tag jquery %}
  - _assets/js/jquery.min.js
  - _assets/js/jquery.cookie.min.js
  {% endjavascript_asset_tag -%}
  "></script>
  ```

  Example of including CSS assets:

  ```liquid
  <link href='
  {%- css_asset_tag fonts %}
  - _assets/css/font-awesome.min.css
  - _assets/css/fonts.css
  {% endcss_asset_tag -%}
  ' rel='stylesheet' type='text/css' crossorigin="anonymous" />
  ```

- If you need to include assets and use a relative link, use the following syntax:

  ```liquid
  {% capture asset_url %}{%- css_asset_tag supported_versions %}[_assets/css/supported_versions.css]{% endcss_asset_tag %}{% endcapture %}
  <link rel="stylesheet" type="text/css" href='{{ asset_url | strip_newlines  | true_relative_url }}' />
  ```

### Dependencies

- Jekyll 4+
- [Jekyll Asset Pipeline](https://github.com/matthodan/jekyll-asset-pipeline)
- [Jekyll Regex Replace](https://github.com/joshdavenport/jekyll-regex-replace)
- [Jekyll Include Plugin](https://github.com/flant/jekyll_include_plugin)

### Jekyll data

Some data is stored in the `_data` directory of the Jekyll project,
while other data is generated from the repo by the scripts or Jekyll hooks.
Below are some data structures used in the Jekyll projects.

- (documentation) `site.data.bundles.raw.[<EDITION>]`. Added in `werf.yaml` to build the followings data in `docs/documentation/_plugins/custom_hooks.rb`:
  - `site.data.bundles.byModule`: A list of bundles for each module. Example:

    ```json
    {
      "node-local-dns": {
        "Default": "true"
      },
      "admission-policy-engine": {
        "Default": "true",
        "Managed": "true"
      }
    }
    ```

  - `site.data.bundles.bundleNames`: A list of available bundles. Example: `["Default", "Managed", "Minimal"]`.
  - `site.data.bundles.bundleModules`: A list of modules for each bundle. Example:

    ```json
    {
      "Default": [
        "node-local-dns",
        "admission-policy-engine"
      ],
      "Managed": [
        "admission-policy-engine",
        "cert-manager"
      ],
      "Minimal": [
        "deckhouse"
      ]
    }
    ```

- `site.data.modules.internal`: A list of embedded modules with the following structure:

  ```text
  {
    "module-name": {
      "path": "A path to the documentation on the site",  <-- null, if the module doesn't have documentation
      "editionMinimumAvailable": "<EDITION>" <-- the "smallest" edition, where module is available. It is computed from the repo folder structure. **Don't use it in logic.** It seems to be deprecated in the future.
    }
  }
  ```

  The data is generated by the `docs/documentation/_tools/modules_list.sh` script.
  
  Example:
  
  ```json
  {
    "admission-policy-engine": {
      "path": "modules/admission-policy-engine/",
      "editionMinimumAvailable": "ce"
    },
    "chrony": {
      "path": "modules/chrony/",
      "editionMinimumAvailable": "ce"
    },
    "cloud-provider-dynamix": {
      "path": "modules/cloud-provider-dynamix/",
      "editionMinimumAvailable": "ee"
    },
    "node-local-dns": {
      "path": "modules/node-local-dns/",
      "editionMinimumAvailable": "be"
    }
  }
  ```

- `site.data.modules.all`: A list of all modules.

  The data is defined by `werf-web.inc.yaml`.
  
  - `editionFullyAvailable`: A list of editions where the module available without restrictions. Used for overriding computed values. Takes precedence over `excludeModules` and `includeModules` from the `site.data.editions` file (see below). The `editionFullyAvailable` for a module can be set in the `docs/documentation/_data/modules/modules-addition.json` file. It's recommended that you don't use it in logic (but you can use it for adding editions to the module).
  - `editionsWithRestrictions`: A list of editions where the module is available with restrictions. Used for overriding computed values. Takes precedence over `excludeModules` and `includeModules` from the `site.data.editions` file (see below). Takes precedence over `editionFullyAvailable`. The `editionsWithRestrictions` for a module can be set in the `docs/documentation/_data/modules/modules-addition.json` file.
  - `editions`: A list of editions where the module is available **with or without** restrictions.
  
  ```text
  {
    "<module-kebab-name>": {
    "editionMinimumAvailable": "<EDITION>",  <-- the "smallest" edition according to the edition weight (_data/modules/editions-weight.yml) where a module is available. It is computed from the module folder of the repo (_tools/modules_list.sh), can be specified in the `_data/modules/modules-addition.json`. **Don't use it in logic.** It seems to be deprecated in the future. Use editions array instead. 
    "editions": [],  <-- a list of editions where the module is available with or without restrictions
    "external": "true|false", <-- Optional, true if the module is installed from the modulesource
    "path": "modules/<module-kebab-name>/",  <-- Optional, path to the module documentation on the site.
    "editionsWithRestrictions": [ <-- editions where the module is available with restrictions
      "se",
      "se-plus",
      "cse-lite"
    ],
    "editionsWithRestrictionsComments": { <-- comments for restrictions. `all` - for all editions
      "all": {
        "en": "Restriction on working with BGP",
        "ru": "Restriction on working with BGP"
      }
    },
    "editionFullyAvailable": [ <-- a list of editions, where the module is available without restrictions. Used for overriding computed values.
      "be",
      "se",
      "se-plus"
    ],  
    "parameters-ee": {  <-- deprecated. A list of parameters for EE
      "some uniq key name": {
        "linkAnchor": "securitypolicy-v1alpha1-spec-policies-verifyimagesignatures",  <-- anchor to the CRD field
        "resourceType": "crd",
        "title": "SecurityPolicy: verifyImageSignatures"
      }
    }
  }
  ```

- `site.data.editions`

  - `docs/documentation/_data/modules/editions-addition.json`: Merged with the data from the `/editions.yaml` file.
  - Each edition in the file can include both `excludeModules` and `includeModules` filters. In this case, the module will be added to the edition if its name is in `includeModules` and not in `excludeModules`.
  - `docs/documentation/_data/modules-addition.json`
  
  ```json
  {
    "ce": {
      "name": "CE",
      "versionMapFile": "candi/version_map.yml",
      "modulesDir": "modules",
      "terraformProviders": [
        "aws",
        "azure",
        "gcp",
        "yandex"
      ],
      "skipFixingImports": true,
      "buildIncludes": {
        "skipCandi": true,
        "skipModules": true
      }
    },
    "be": {
      "name": "BE",
      "versionMapFile": "ee/be/candi/version_map.yml",
      "modulesDir": "ee/be/modules",
      "excludeModules": [
        "openvpn",
        "sds-node-configurator",
        "sds-replicated-volume",
        "sds-local-volume",
        "virtualization",
        "csi-ceph",
        "csi-nfs"
      ]
    },
    "se": {
      "name": "SE",
      "modulesDir": "ee/se/modules",
      "excludeModules": [
        "dashboard"
      ]
    },
    "se-plus": {
      "name": "SE+",
      "modulesDir": "ee/se-plus/modules",
      "terraformProviders": [
        "vsphere",
        "ovirt"
      ],
      "excludeModules": [
        "cloud-provider-dynamix",
        "cloud-provider-openstack",
        "cloud-provider-vcd",
        "dashboard",
        "keepalived",
        "network-gateway",
        "operator-trivy",
        "runtime-audit-engine",
        "static-routing-manager",
        "virtualization"
      ],
      "languages": [
        "ru"
      ],
      "includeModules": [
        "cloud-provider-vsphere",
        "cloud-provider-zvirt"
      ]
    },
    "ee": {
      "name": "EE",
      "modulesDir": "ee/modules",
      "terraformProviders": [
        "openstack",
        "vcd",
        "decort",
        "huaweicloud"
      ]
    },
    "fe": {
      "name": "FE",
      "modulesDir": "ee/fe/modules"
    },
    "cse-lite": {
      "name": "CSE Lite",
      "languages": [
        "ru"
      ],
      "excludeModules": [
        "basic-auth",
        "cert-manager",
        "cilium-hubble",
        "cloud-provider-aws",
        "cloud-provider-azure",
        "cloud-provider-dynamix",
        "cloud-provider-gcp",
        "cloud-provider-openstack",
        "cloud-provider-vcd",
        "cloud-provider-vsphere",
        "cloud-provider-yandex",
        "cloud-provider-zvirt",
        "cni-simple-bridge",
        "commander",
        "commander-agent",
        "console",
        "csi-ceph",
        "csi-nfs",
        "dashboard",
        "deckhouse-tools",
        "delivery",
        "descheduler",
        "documentation",
        "extended-monitoring",
        "external-module-manager",
        "flant-integration",
        "istio",
        "keepalived",
        "metallb-crd",
        "monitoring-custom",
        "monitoring-custom",
        "monitoring-ping",
        "multitenancy-manager",
        "namespace-configurator",
        "network-gateway",
        "network-policy-engine",
        "node-local-dns",
        "okmeter",
        "openvpn",
        "operator-ceph",
        "operator-postgres",
        "pod-reloader",
        "prometheus-pushgateway",
        "sds-drbd",
        "sds-elastic",
        "sds-local-volume",
        "sds-node-configurator",
        "sds-replicated-volume",
        "secret-copier",
        "secrets-store-integration",
        "static-routing-manager",
        "stronghold",
        "terraform-manager",
        "upmeter",
        "vertical-pod-autoscaler",
        "virtualization"
      ]
    },
    "cse-pro": {
      "name": "CSE Pro",
      "languages": [
        "ru"
      ],
      "excludeModules": [
        "basic-auth",
        "cert-manager",
        "cilium-hubble",
        "cloud-provider-aws",
        "cloud-provider-azure",
        "cloud-provider-dynamix",
        "cloud-provider-gcp",
        "cloud-provider-openstack",
        "cloud-provider-vcd",
        "cloud-provider-vsphere",
        "cloud-provider-yandex",
        "cloud-provider-zvirt",
        "cni-simple-bridge",
        "commander",
        "commander-agent",
        "console",
        "dashboard",
        "deckhouse-tools",
        "delivery",
        "descheduler",
        "extended-monitoring",
        "external-module-manager",
        "flant-integration",
        "istio",
        "keepalived",
        "monitoring-custom",
        "monitoring-ping",
        "namespace-configurator",
        "network-gateway",
        "network-policy-engine",
        "okmeter",
        "openvpn",
        "operator-ceph",
        "operator-postgres",
        "pod-reloader",
        "prometheus-pushgateway",
        "sds-drbd",
        "sds-elastic",
        "sds-local-volume",
        "sds-node-configurator",
        "sds-replicated-volume",
        "secret-copier",
        "secrets-store-integration",
        "static-routing-manager",
        "stronghold",
        "terraform-manager",
        "upmeter",
        "virtualization"
      ]
    }
  }
  ```

## Search

This feature allows you to display a contextual message above the "ready" search message to inform users about what they're searching in.

### Usage

```html
<input type="text" id="search-input" 
       placeholder="Search..." 
       class="input"
       data-search-index-path="/path/to/search.json"
       data-search-context="Searching in modules documentation"> 
```

### Examples

#### Modules Documentation
```html
<input type="text" id="search-input" 
       placeholder="Search modules..." 
       class="input"
       data-search-index-path="/modules/search-embedded-modules-index.json"
       data-search-context="Searching in modules documentation">
```

#### Platform Documentation
```html
<input type="text" id="search-input" 
       placeholder="Search..." 
       class="input"
       data-search-index-path="/search.json"
       data-search-context="Searching in platform documentation and modules">
```

#### Product-Specific Documentation
```html
<input type="text" id="search-input" 
       placeholder="Search..." 
       class="input"
       data-search-index-path="/products/kubernetes-platform/documentation/search.json"
       data-search-context="Searching in Kubernetes Platform documentation">
```

### Behavior

- The context message only appears when the search is ready and no query has been entered
- It appears above the "What are we looking for?" message
- If no `data-search-context` attribute is provided, the normal ready message is displayed
- The context message is hidden when search results are shown

### Internationalization

Jekyll/Liquid:

```html
data-search-context="{{ site.data.i18n.search.context[page.lang] }}"
```

Hugo:

```html
data-search-context="{{ T "search_context" }}"
```
