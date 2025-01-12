# Deckhouse documentation website 

> May contain incomplete information. It is being updated.

This document describes Deckhouse documentation architecture and how to run the documentation website locally.

## Running a site with the documentation locally

### Requirements

- Clone repo firstly.
- Free 80 port to bind.
- Install [werf](https://werf.io/getting_started/).

### Starting and stopping a site with the documentation locally — the first method

- To start documentation, open two separate consoles and follow the steps:

  1. In the first console run:

     ```shell
     cd docs/documentation
     make up
     ```

  1. In the second console run:

     ```shell
     cd docs/site
     make up
     ```

  1. Open <http://localhost>

- To stop documentation, cancel the process in the consoles and run:

  ```shell
  make down
  ```

### Starting and stopping a site with the documentation locally — the second method

- To start documentation, open console and follow the steps:

  1. In the console run:

     ```shell
     make docs
     ```

  1. Open <http://localhost>

- To stop documentation, cancel the process in the console and run:

  ```shell
  make docs-down
  ```

## How to debug

> Instructions may be outdated!

There is the `docs/site/werf-debug.yaml` file to compile and the `docs/site/docker-compose-debug.yml` file to run the backend with [delve](https://github.com/go-delve/delve) debugger.

Run from the docs/site directory of the project (or run docs/site/backend/debug.sh):

```shell
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'
```

Connect to localhost:2345

## Working with spellchecker

Commands below assume that you are run them from the root of the repository.

Use the following commands:
- `make docs-spellcheck` — to check all the documentation for spelling errors.
- `file=<PATH_TO_FILE> make docs-spellcheck` — to check the specified file for spelling errors.

  Example:

  ```shell
  file=ee/se-plus/modules/cloud-provider-vsphere/docs/CONFIGURATION_RU.md make docs-spellcheck`
  ```

- `make docs-spellcheck-generate-dictionary` — to generate a dictionary of words. Run it after adding new words to the tools/docs/spelling/wordlist file.
- `make docs-spellcheck-get-typos-list` — to get the sorted list of typos from the documentation.

The `make lint-doc-spellcheck-pr` command is used in CI to check the spelling of the documentation in a PR.

## Architecture

There following parts of the Deckhouse website:
- The main part of the site. Includes all sections except that not described below.
- The non-versioned documentation part.
   
  Includes the following sections:
  - `/products/kubernetes-platform/gs/`
  - `/products/kubernetes-platform/guides/`
  - `/assets/`
  - `/images/`
  - `/presentations/`
  - `/products/virtualization-platform/documentation/`
  - `/products/virtualization-platform/gs/`
  - `/products/virtualization-platform/guides/`
  - `/products/virtualization-platform/reference/`
  
  Content is generated using Jekyll from the `docs/site` directory.
  
- The versioned documentation part. 

  Includes the following sections:
  - `/products/kubernetes-platform/documentation/`

  Content is generated using Jekyll from the `docs/documentation` directory.

  Contains documentation for Deckhouse Kubernetes Platform and built-in modules.

- Documentation for Deckhouse Kubernetes Platform modules.

  Includes the following sections:
  - `/products/kubernetes-platform/modules/`

  Content is generated using HuGo:
  - Project files for HuGo is in the `docs/site/backends/docs-builder-template` directory.
  - The builder, which generates the documentation, is in the `docs/site/backends/docs-builder` directory (written in Go).

### Structure of the Jekyll-based projects

Project uses [werf](werf.io) to build and deploy documentation.

Some tips:
- `_tool` directory contains scripts used for build;
- `_assets` directory stores assets (styles and scripts), which are used by Jekyll Asset Pipeline plugin. Assets are compiled and minified into the `/assets` directory (yeah, the absolute path) and have a digest in the path. If you don't need a digest in the path, you may use `/css` or `/js` directory (assets will be processed by Jekyll as usual).  
  
  Here is an example of how to include a JavaScript assets:

  ```liquid
  <script type="text/javascript" src="
  {%- javascript_asset_tag jquery %}
  - _assets/js/jquery.min.js
  - _assets/js/jquery.cookie.min.js
  {% endjavascript_asset_tag -%}
  "></script>
  ```

- Here is an example of how to include a CSS assets:

  ```liquid
  <link href='
  {%- css_asset_tag fonts %}
  - _assets/css/font-awesome.min.css
  - _assets/css/fonts.css
  {% endcss_asset_tag -%}
  ' rel='stylesheet' type='text/css' crossorigin="anonymous" />
  ```

- If you need to include assets and use a relative link, you can use the following syntax:

  ```liquid
  {% capture asset_url %}{%- css_asset_tag supported_versions %}[_assets/css/supported_versions.css]{% endcss_asset_tag %}{% endcapture %}
  <link rel="stylesheet" type="text/css" href='{{ asset_url | strip_newlines  | true_relative_url }}' />
  ```

### Dependencies
- Jekyll 4+
- [Jekyll Asset Pipeline](https://github.com/matthodan/jekyll-asset-pipeline)
- [Jekyll Regex Replace](https://github.com/joshdavenport/jekyll-regex-replace]
- [Jekyll Include Plugin](https://github.com/flant/jekyll_include_plugin)

### Jekyll data

Some data is stored in the `_data` directory of a Jekyll project, but some data is generated from the repo by the scripts or by jekyll hooks. Here are some data structures, which are used in the Jekyll projects. 

- (documentation) `site.data.bundles.raw.[<EDITION>]`. Added in the werf.yaml to build the followings data (in `docs/documentation/_plugins/custom_hooks.rb`):
  - `site.data.bundles.byModule` — list of bundles for each module. Example: 
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
  - `site.data.bundles.bundleNames` — list of available bundles. Example: `["Default", "Managed", "Minimal"]`
  - `site.data.bundles.bundleModules` — list of modules for each bundle. Example:
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
- `site.data.modules.internal`. List of embedded modules with the following structure:
  ```text
  {
    "module-name": {
      "path": "path to the documentation on the site",  <-- null, if the module don't have documentation
      "editionMinimumAvailable": "<EDITION>" <-- the "smallest" edition, where module is available. It is computed from the repo folder structure.
    }
  }
  ```
   
  The data is filled by the `docs/documentation/_tools/modules_list.sh` script.
  
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
    
- `site.data.modules.all`. List of all the modules.
 
  The data is filled in the `werf-web.inc.yaml`.
  
  - `editionFullyAvailable` - he list of editions, where the module is available without restrictions. Used for overriding computed values. Takes precedence over `excludeModules` and `includeModules` from the `site.data.editions` file (see below).
  - `editionRestrictions` - The list of editions, where the module is available with restrictions. Used for overriding computed values. Takes precedence over `excludeModules` and `includeModules` from the `site.data.editions` file (see below). Takes precedence over `editionFullyAvailable`.
  - `editions` — The list of editions, where the module is available **with or without** restrictions. 
  
  ```text
  {
    "<module-kebab-name>": {
    "editionMinimumAvailable": "<EDITION>",  <-- the "smallest" edition, where module is available. It is computed from the repo folder structure.
    "editions": [],  <-- list of editions, where the module is available with restrictions or without restrictions
    "external": "true|false", <-- true if the module installs from the modulesource
    "path": "modules/<module-kebab-name>/",  <-- path to module documentation on the site (on null)
    "editionRestrictions": [ <-- editions, where the module is available with restrictions
      "se",
      "se-plus",
      "cse-lite"
    ],
    "editionRestrictionsComment": { <-- comments for restrictions. `all` - for all editions
      "all": {
        "en": "Restriction on working with BGP",
        "ru": "Ограничение на работу с BGP"
      }
    },
    "editionFullyAvailable": [ <-- list of editions, where the module is available without restrictions. Used for overriding computed values.
      "be",
      "se",
      "se-plus"
    ],  
    "parameters-ee": {  <-- deprecated. list of parameters for EE
      "some uniq key name": {
        "linkAnchor": "securitypolicy-v1alpha1-spec-policies-verifyimagesignatures",  <-- anchor to the CRD field
        "resourceType": "crd",
        "title": "SecurityPolicy: verifyImageSignatures"
      }
    }
  },
  ```
- `site.data.editions`

   - `docs/documentation/_data/editions-addition.json` - the data from the file is merged into the data from the `/editions.yaml` file. 
   - Each edition data can include both filters - `excludeModules` and `includeModules`. In this case, the module will be added to edition if it its name is in the `includeModules` and does not in the `excludeModules`. 

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
        "ceph-csi",
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
        "ceph-csi",
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
        "snapshot-controller",
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
        "ceph-csi",
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
