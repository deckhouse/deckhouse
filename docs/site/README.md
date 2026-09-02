# Deckhouse documentation website

> This README is a work in progress. Some information may be incomplete or outdated.

This document describes the architecture of Deckhouse documentation and explains how to run the documentation website locally.

## Running the documentation site locally

### Requirements

- Clone this repository.
- Ensure that port `80` is available for binding.

### Running the documentation site (option 1: watch mode)

Runs documentation containers in a werf watch mode — documentation will rebuild on a new commit (on `make up`) or any changes (on `make dev`).

#### Starting the documentation site

To start the documentation site follow these steps:

1. Run documentation:

   ```shell
   cd docs/site
   make up
   ```

   By default, local Docker images store is used (no registry required).

   If you want to use a local Docker registry at `localhost:4999` instead, set `USE_LOCALHOST_REPO=1`.
   In that case, the registry is started automatically if it is not already running:

   ```shell
   cd docs/site
   USE_LOCALHOST_REPO=1 make up
   ```

   If you want to work with uncommitted files, use `make dev` instead of `make up`.

1. Open the DKP documentation in your browser at <http://localhost/products/kubernetes-platform/documentation/v1/>.

#### Stopping the documentation site

To stop the documentation site, cancel the running process and run the following command:

```shell
make down
```

This also stops and removes the local Docker registry if it was started.

### Running the documentation site with an external module

Use this mode when you want to preview documentation from an external module repository together with the local portal.

1. Run the following command from `docs/site`:

   ```bash
   make external-module MODULE_PATH=/path/to/module
   ```

1. Optional arguments:

   - `CHANNEL` — defaults to `alpha`;
   - `MODULE_VERSION` — defaults to `v0.1.0`.

   Example:

   ```bash
   make external-module \
     MODULE_PATH=/home/kar/fox/platform-security/operator-trivy \
     CHANNEL=stable \
     MODULE_VERSION=v1.2.3
   ```

1. Open the DKP documentation in your browser at <http://localhost/products/kubernetes-platform/documentation/v1/>.

The external module pages are available under `/modules/<module-name>/<channel>/`.

If you edit files in the external module repository, Hugo rebuilds the generated files automatically and the portal picks up the changes.

The workflow watches:

- `docs/`;
- `module.yaml`;
- `oss.yaml`;
- `openapi/config-values.yaml`;
- `openapi/doc-ru-config-values.yaml`;
- root-level files in `crds/`.

YAML files from subdirectories inside `crds/` are ignored.

#### Stopping the external module workflow

To stop the workflow, cancel the running process and then run:

```bash
make down
```

### Running the documentation site (option 2: just run containers)

Just runs documentation containers.

#### Starting the documentation site

To start the documentation site, open a terminal and follow these steps:

1. Run the following command in the repository root:

   ```shell
   make docs
   ```

1. Open the documentation site in your browser at <http://localhost/products/kubernetes-platform/documentation/v1/>.

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

## Verifying an external module build (module author workflow)

Use this mode from within an external module repository to check that its
`docs/` directory renders successfully with the Deckhouse Hugo template. The
check runs Hugo in Docker — no local Deckhouse clone or Hugo install is
required.

Requirements on the machine running the check:

- `bash`, `docker`, `yq` on `PATH`;
- `git` (only when the script has to fetch the Deckhouse repo itself);
- network access to `github.com` and `ghcr.io`.

Run from the module repository root:

```bash
curl -sSfL https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/docs/check-external-module.sh \
  | bash -s -- --module-path .
```

Or download and invoke explicitly:

```bash
curl -sSfL https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/docs/check-external-module.sh \
  -o /tmp/check-external-module.sh
bash /tmp/check-external-module.sh --module-path "$(pwd)" --channel alpha
```

Useful flags: `--channel`, `--version`, `--output <dir>`, `--keep`,
`--deckhouse-repo <path>` (reuse a local checkout), `--deckhouse-ref <ref>`.
Run `check-external-module.sh --help` for the full list.

GitHub Actions example:

```yaml
jobs:
  docs:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install yq
        run: sudo snap install yq
      - name: Fetch Deckhouse docs checker
        run: |
          curl -sSfL https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/docs/check-external-module.sh \
            -o /tmp/check-external-module.sh
          chmod +x /tmp/check-external-module.sh
      - name: Verify module docs build
        run: /tmp/check-external-module.sh --module-path "$(pwd)"
```

## Debugging (WIP)

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

## Architecture (WIP)

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

> Some information is outdated.

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

> Some information is outdated.

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
        ...,
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
        ...,
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
        ...,
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
        ...,
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

### Result blocks and pagination

Results are rendered in three blocks in a fixed order — Modules, API (OpenAPI parameters and resources) and Documentation (pages) — and nothing moves a result from one block to another.

- The API and Documentation blocks show 5 results each; the "show more" button below a block adds 5 more (`pageSize` in `search-v3.js`). The button is not rendered once the block has nothing left.
- The button also states how much is left in its own block — «Показать еще 5 (осталось 43)» — so the number of clicks to the end of the list is visible. On the last click the remainder equals the batch, and the label drops the parentheses.
- Inside the API block results are ordered by four internal priorities (resource name match, parameter name match, other resources, other parameters), but the block is paginated as a single list — one counter, one button.
- The Modules row shows up to 14 badges and then `... and N more` as plain text, without a way to expand it.
- The search itself is not limited: Lunr returns every match and grouping keeps them all, so a block may hold hundreds of results with 5 of them rendered. Only the rendering is capped.

### Ranking a single page (`searchBoost`)

Search results are scored by Lunr (field weights: `title` 10, `keywords` 9, `module` 6, `summary` 3, `content` 1) and then adjusted by `search-v3.js`. To move one specific page up or down without touching those global weights, set `searchBoost` in its front matter:

```yaml
---
title: "Overview"
searchBoost: 1.5
---
```

Behavior:

- The value is a multiplier applied to the page's score. `> 1` promotes, `< 1` demotes (`0.3` is a good starting point for release notes and other low-value pages).
- The value is not capped. A missing, zero, negative or non-numeric value is ignored (multiplier `1`), so a typo cannot break the search — but a wrong *number* is applied as written, so keep an eye on it in review. Note that the value is a multiplier, not a percentage: `searchBoost: 300` is enormous, not "300 %".
- Start at `2` and check the results. A page competing only on `content` matches usually needs `1.5`–`3`. Beating a competitor that matches in `title` takes roughly `40`+ — in that case add `search` keywords instead (see below), which is both cheaper and less disruptive.
- It **reorders results within a group only**. Results are rendered as Modules → API → Documentation, and that order is fixed — no boost will lift a documentation page above the API block.
- It reorders matches, it does not create them. A page that the query does not match at all will not appear no matter how high the boost.
- Works in both generators: `page.searchBoost` for Jekyll (`docs/documentation`), `.Params.searchBoost` for Hugo (external modules).
- Applies to pages only. OpenAPI parameters have no front matter — use `x-doc-search` keywords for those.

The related key `search` (comma-separated keywords) is often the better first tool: the `keywords` field already carries a weight of 9 and picks up an extra multiplier during post-processing. Reach for `searchBoost` when the page already matches and simply needs to rank higher.

### Query syntax

The search box takes plain text. Lunr's query language is not exposed to visitors: every operator is rewritten before the query reaches the index by the `LUNR_SYNTAX_RULES` table in `docs/site/assets/js/search-v3.js`. That file is the only owner of query syntax — it sanitizes the query before handing it to the worker in the `SEARCH` message (along with a `requireAllWords` flag for the `key: value` case) and sanitizes the synonym map it sends on `INIT`, so `search-v3-worker.js` never sees raw input and needs no copy of the table. The rules are idempotent, because `buildPhraseQuery()` re-adds `+` afterwards.

| Typed | Searched | Note |
|---|---|---|
| `+ingress`, `-nginx`, `install --dry-run` | `ingress`, `nginx`, `install dry-run` | presence operators are inert in any script |
| `ingress~5` | `ingress` | fuzzy matching is off: an edit distance of 5 matched 2464 pages in 76 ms |
| `ingress^10 nginx` | `ingress nginx` | boosting is off: relevance belongs to the field boosts |
| `kind: configmap`, `content:nginx` | `+kind +configmap`, `+content +nginx` | see below |
| `ingres*` | `ingres*` | **the one supported operator** |
| `a*`, `*gress`, `in*ss` | `a`, `gress`, `in ss` | only a trailing `*` on a term of 3+ characters survives; a leading one cost 111 ms against 2 ms, and `*` alone meant the whole corpus (7154 pages, 993 ms) |
| `*`, `+`, `:` | — | a query made only of operators sanitizes to nothing and reports "not found"; an empty Lunr query would otherwise return every page |

A colon is read as a `key: value` pair pasted from a manifest, so **both parts are required**: `kind: configmap` matches 29 pages where the same words OR-ed match 381. `kind:configmap` and `kind: configmap` are the same query — the index holds the token `kind`, not `kind:`, because `lunr.trimmer` strips it. If nothing contains all the words, the search falls back to the ordinary OR query, so a pair is never a dead end. This also removes a trap: `content:`, `title:`, `module:`, `summary:` and `keywords:` are real index fields, and such a query used to be silently scoped to one field (`content:nginx` returned 107 pages instead of 218, with nothing in the UI to show it).

Malformed input cannot break the search: `searchWithFallback()` retries once with every non-word character stripped, and a failure after that shows an error message instead of an empty dropdown.

### Synonyms (`synonymGroups`)

Synonyms let a query find pages that do not contain its words at all — for example, «провайдеры аутентификации» finds the `DexProvider` resource. They live in `options.synonymGroups` in `docs/site/assets/js/search-v3.js` as groups of equivalent terms:

```js
synonymGroups: [
  ['moduleupdatepolicy', 'update policy', 'module update policy', 'политика обновления'],
  ['dexprovider', 'dex providers', 'провайдеры аутентификации'],
],
```

Every member of a group expands to all the others, so a link works in both directions with no reverse entries to maintain. If a term must expand *without* the reverse link, use the `options.synonyms` map instead (`{ 'what the user types': ['extra query'] }`); it is merged on top of the groups.

Editing rules:

- `search-v3.js` is the only place to edit. The search itself runs in `search-v3-worker.js`, but the worker receives the derived map with the `INIT` message and keeps no list of its own — a group added there has no effect.
- Case and extra spaces do not matter: terms are normalized (lowercased, whitespace collapsed) and sanitized once, when the map is built, and lookups use the same normalization. Keep them lowercase anyway, for consistency with the existing groups.
- Expansion is matched against the whole query and against every window of up to 4 consecutive words in it, so `update policy for a module` expands as well as `update policy`.

Behavior:

- Each expansion is run as a separate Lunr query and its results are merged into the original set with a 1.15 multiplier. Synonyms therefore add and reorder results — they do not turn an irrelevant page into a match.
- A multi-word synonym is matched as a whole. Lunr has no phrase queries, so the expansion is rewritten into required terms (`Security Information and Event Management` → `+security +information +event +management`) and a page has to contain all of them. Passed as a plain string it would be an OR over the words: 1031 hits instead of 3, everything that merely says «management». Stop words (`and`, `и`, `в`) are left out of such a query — they are dropped when the index is built, so requiring one empties the result set.
- Every applied term is highlighted, not just what the user typed: searching «провайдеры аутентификации» marks `DexProvider` in titles, breadcrumbs and snippets, and the snippet itself is picked by coverage of all the terms, so a sentence mentioning only the synonym still wins.
- A synonym is highlighted exactly as written, never word by word: `siem` marks «Security Information and Event Management» and leaves a stray «management» alone. Words of the *query* are still highlighted separately, because Lunr does match them independently. The trade-off: a page found through a synonym whose phrase it does not contain literally gets a snippet with nothing marked.
- Highlighting tolerates inflections (a Russian query «провайдеры» also marks «провайдеров»), prefers whole phrases over separate words, and anchors matches at word starts.

### OpenAPI Specifications rendering

The `x-doc-` prefix in the parameter names is reserved in the OpenAPI specifications for rendering the documentation. Parameters with this prefix are only used for rendering the documentation and are not mandatory.
A list of `x-doc-` parameters:
- `x-doc-deprecated:` (boolean). It is used to indicate that the parameter is deprecated.
- `x-doc-required:` (boolean). It is used to indicate explicitly on the site if a particular parameter is mandatory or optional.
- `x-doc-default:` (arbitrary type). The default value to show on the site. It is helpful if you cannot specify the `default` parameter for some reason. The x-doc specification value must be of the same type as the target parameter, and it **cannot contain** markdown elements or arbitrary text (well, it can, but the rendering will be ugly). **Only** the value from the English version of the resource is used.
- `x-doc-d8Editions` (array of strings). Array of Deckhouse Kubernetes Platform editions the target parameter can be used with. E.g. `["se", "ee"]`. Legacy, and will be deprecated.
- `x-doc-example` (arbitrary type). Provides an example of the target parameter's value. If specified, it takes precedence over the `example` and `x-examples` parameters. The x-doc-example specification value can contain markdown elements or arbitrary text. **Only** the value from the English version of the resource is used. Use `x-doc-examples` for specifying an array of YAMLs.
- `x-doc-examples` (arbitrary type). Provides an ARRAY of examples of the target parameter's value. If specified, it takes precedence over the `example` and `x-examples` parameters.
- `x-doc-search` (string). Comma-separated search keywords. Are used in the search index on the site to search parameters better.
- `x-doc-skip` (boolean). If true, skip parameter for rendering.
- `x-doc-map-key-name` (string). Used to specify the name of an additional parameter (object key) when describing `additionalProperties`.
- `x-doc-pattern-name` (string). Used to specify the name of a pattern inside `patternProperties` object.

## AI-friendly exports (llms.txt and corpus.json)

Alongside the HTML, every build publishes the same content in a form an LLM agent can consume:

- a Markdown copy of each page, next to its HTML (`…/faq.html` → `…/faq.md`);
- an [llms.txt](https://llmstxt.org/) index — the page list grouped by the site navigation, with a description and a link to the `.md` for each entry;
- a `corpus.json` for RAG — the same pages with their metadata, full Markdown and pre-split chunks. It carries its own JSON Schema (draft 2020-12) under the top-level `schema` key, so the file describes itself.

The exports are built from the *rendered* HTML, not from the source Markdown: includes, shortcodes and the OpenAPI schemas rendered by `render-jsonschema.rb` are already expanded there, and an agent gets the page as a reader sees it.

Each `.md` is made to stand on its own, so it can be read — or split into chunks — without ever going back to the HTML:

- a **YAML frontmatter** header carries the page's provenance (`title`, `canonical`, `lang`, and, when they apply, `version`, `module`, `moduleType`, `channel`, `editions`, `stage`). The field names match the `corpus.json` document, so the two artifacts share one vocabulary. The corpus keeps the bare body (and hashes it); the frontmatter fields are already columns there, so they are not repeated inside its `markdown`.
- **internal links point at the `.md` twin** of the target page: `…/faq.html` → `…/faq.md`, and a directory link `…/foo/` → `…/foo/index.md`. This applies to same-site links whether they are root-relative or written out in full (`https://deckhouse.io/…`); links to other hosts are left alone. The rewrite is unconditional (it does not check that the target `.md` exists), which is what keeps a link from one build into another — an embedded module page into the documentation — working. Both generators apply the directory → `index.md` step under the documentation and modules templates (`/products/*/documentation/` and `/modules/`), leaving any other directory link (e.g. `/downloads/…/`) on its HTML URL. For the target file to actually be `index.md`, the external-modules export (Hugo) normalizes a module's directory README: `README.md` renders to `readme.html`, but its Markdown twin is written as `index.md` — unless the directory already has an authored `index.html`, in which case the README keeps `readme.md` so it does not clobber that index. The Jekyll embedded modules keep `readme.md` and rely on the web server to redirect `index.md` → `readme.md`.
- **headings carry their HTML anchor** as a kramdown/Pandoc attribute (`## Title {#id}`), so an in-page link (`cr.md#customresourcedefinition`) resolves. The text alone is not enough — for Cyrillic the HTML slug is not what a Markdown renderer would derive.

### What is published where

| Content | Generator | Files |
| --- | --- | --- |
| DKP documentation and built-in modules | Jekyll, `docs/documentation` | `/products/kubernetes-platform/documentation/v1/{llms.txt,corpus.json}` |
| Embedded modules library | Jekyll, the `modules-embedded` build of the same sources | `/modules/{embedded-llms.txt,embedded-corpus.json}` |
| External modules library | Hugo + docs-builder | `/modules/{external-llms.txt,external-corpus.json}` |

The documentation `llms.txt` is the entry point: it links to the two module indexes and to all three corpora.

### Jekyll (`_plugins/ai_export.rb`)

The plugin runs on the `site, :post_write` hook — `page.output` is complete only there, while `page.content` inside Liquid depends on the page render order.

All settings live under the `ai_export` mapping in the site config (not to be confused with the per-page `ai_export: false` front-matter flag, which excludes a page):

| Parameter | Meaning |
| --- | --- |
| `ai_export.enabled` | Enables the export. Nothing is generated unless it is `true`. |
| `ai_export.root` | Adds the `Optional` section with the links to the corpora. Set it in the build whose llms.txt is the entry point of the site. |
| `ai_export.llmsFileName` | Name of the llms.txt file. Default: `llms.txt`. |
| `ai_export.corpusFileName` | Name of the corpus file. Default: `corpus.json`. |
| `ai_export.title` | The `# heading` of llms.txt. Accepts a per-language hash. Falls back to `site_title`. |
| `ai_export.summaryI18nKey` | Name of an `i18n.common` entry (`_data/i18n.yml`, per-language) to use as the `> summary` of llms.txt. Lets each build point at its own text — e.g. the embedded-modules build at a module-specific summary. Falls back to `site_description` when unset or empty. |

The file names are parameters because `docs/documentation` is built more than once and the results are published side by side. The main build takes them from `_config.yml`, the embedded modules build deep-merges its overrides into the `ai_export` block via `/tmp/_config_additional.yml` (see `werf-modules-static.inc.yaml`).

A page is exported if it is `searchable: true`, or if it is a `CONFIGURATION`/`CR`/`CLUSTER_CONFIGURATION` page generated from an OpenAPI schema — those are dropped from the search index but are the most valuable reference an agent can get. Set `ai_export: false` in the front matter (or in `defaults` of `_config.yml`, the way `pages/internal` and `pages/drafts` do) to keep a page out.

### External modules (Hugo + docs-builder)

Hugo does not write the export itself. The `ai` output format renders a manifest, `<lang>/ai/ai.json`, listing every module page with its metadata; the Go exporter (`backends/docs-builder/internal/aiexport`) then converts the rendered HTML and writes the `.md` files, `external-llms.txt` and `external-corpus.json`. In a cluster this is a step of the docs-builder build.

The names here are fixed rather than configurable: the modules library shares its URL space with the documentation, so the artifacts have to be told apart by name.

### The embedded schema

Every `corpus.json` embeds its own JSON Schema under the top-level `schema` key. The schema is generated by reflection from the Go structs that produce the data (`Corpus`/`Document`/`Chunk` in `internal/aiexport`, with per-field `desc` tags), so it cannot drift from what the exporter writes. The Go exporter builds it at run time; the Jekyll exporter embeds a checked-in copy at `docs/documentation/_data/corpus_schema.json`, which `internal/aiexport/schema_test.go` keeps equal to the structs — regenerate it after any struct change with:

```shell
go test ./internal/aiexport -run TestCorpusSchema -update
```

Both generators emit the same document shape (optional fields — `module`, `moduleType`, `version`, `stage`, `channel`, chunk `anchor` — are omitted, never `null`), so the one schema validates the output of both. `TestExport` checks that the corpus the Go side writes validates against the schema it embeds.

### Generating the external modules export locally

```shell
make ai-export
```

docs-builder is an HTTP service that expects a Kubernetes API, so the target does not run it: it renders the site with Hugo and calls the exporter directly over the result. It expects a prepared `content/modules` + `data/modules` tree — by default `backends/docs-builder-template`, override with `AI_EXPORT_SRC=/path/to/tree`. A module missing from `data/modules/channels.yaml` is silently skipped, as it is on the site.

### Serving

The artifacts are static files, but they are published under URLs that do not match their location on disk, so every nginx config has to know about them: `.werf/nginx-local.conf` (local site), `.helm/templates/10-cm-moduleslibrary.yaml` (deckhouse.io) and `modules/810-documentation/templates/configmap.yaml` (the documentation module in a cluster). The per-page `.md` files are served by the generic page locations; the configs also declare `text/markdown` for `.md`, otherwise nginx offers them for download.

## Markup (external modules documentation)

[Hugo](gohugo.io) SSG is used for rendering.

The documentation content is written in Markdown with some custom shortcodes.

### Page parameters (front matter)

#### Related links

```yaml
params:
  relatedLinks:
    - title: "Link"
      url: link.html
    - title: "External link"
      url: "http://domain/external/link.html"
    - url: /modules/monitoring-kubernetes/
```

### Shortcodes

<div id="alert-details"></div>

#### Alert

There are following levels of alerts: `info`, `warning`, `danger`. The default level is `info`.

```go
{{< alert level="warning" >}}
The warning message...
{{< /alert >}}
```

#### Tabs

```go
{{< tabs name="tabs_uniq_name" >}}
{{% tab name="Tab caption 1" %}}Tab 1 Content {{% /tab %}}
{{% tab name="Tab caption 2" %}}Tab 2 Content {{% /tab %}}
{{< /tabs >}}
```

#### Translate

Translates content based on the current language using the translations defined in the `i18n` folder.

```go
{{< translate "version_of_module" >}}
```

<div id="shortcode-details"></div>

#### Details

```go
{{% details "Summary..."%}}
## Markdown content

Markdown content...
{{% /details %}}
```

### Partials

#### Details

The same as the [details shortcode](#user-content-shortcode-details), but used in templates.

```
{{ partial "details" ( dict "summary" "Summary..." "content" "Markdown content..." ) }}
```

#### Alert

The same as the [alert shortcode](#user-content-alert-details), but used in templates.

```
{{ partial "alert" ( dict "level" "warning" "content" "Markdown content..." ) }}
```

## PDF generation

Documentation PDFs are produced in two independent ways:

- a user can export the current documentation page in the browser;
- maintainers can generate complete administrator and user guides with `make docs-generate-pdf`.

### Exporting a page in the browser

Browser PDF export is currently enabled only for pages in `pages/guides`. The default configuration sets `allowPDFDownload: true` for these pages, which displays a localized **Download page as PDF** button.

The export can be enabled explicitly on another page by adding the following front matter:

```yaml
allowPDFDownload: true
```

Set `allowPDFDownload: false` on a guide page to disable the button.

The button is rendered by `_includes/pdf-download-button.html` in the `guide`, `page`, and `sidebar-guides` layouts. When the setting is disabled, the button and the PDF export script are not added to the page.

The export runs entirely in the browser:

1. `assets/js/pdf-export.js` clones the `.post-content` element and removes interactive controls that must not appear in the PDF.
1. Relative links are converted to absolute URLs. Images are converted to embedded PNG data URLs because pdfmake cannot load page images while creating the document.
1. `html-to-pdfmake.min.js` converts the HTML into a pdfmake document definition.
1. `pdfmake.min.js` downloads the result using a file name derived from the page title.

The generated document contains a title page, the source URL, the generation time, a running header, and page numbers. The export also applies print-specific formatting:

- alerts and blockquotes use colored callout boxes;
- expanded `details` content uses a gray callout box, and its summary is rendered as text rather than a link;
- code blocks, inline code, headings, nested lists, tables, and images are normalized for pdfmake;
- colored square status emoji are rendered as vector shapes;
- headings are kept with the content that follows them when possible.

pdfmake and html-to-pdfmake are loaded only after the first click. The DejaVu Sans files under `assets/fonts/dejavu/` are also loaded on demand and registered as the default PDF font. DejaVu Sans provides Cyrillic, box-drawing characters, and common symbols used in command output. The four font files provide regular, bold, oblique, and bold-oblique styles.

The asset version from `head-site.html` is appended to lazy-loaded scripts and fonts for cache invalidation. If a library or font fails to load, the button is enabled again so the user can retry the export.

### Generating complete guides

The batch generator creates an administrator's guide and a user's guide, each in English and Russian.

#### Output files

| File | Content |
|------|---------|
| `pdf/deckhouse-admin-guide_en.pdf` | Administrator's guide, English |
| `pdf/deckhouse-admin-guide_ru.pdf` | Administrator's guide, Russian |
| `pdf/deckhouse-user-guide_en.pdf` | User's guide, English |
| `pdf/deckhouse-user-guide_ru.pdf` | User's guide, Russian |

#### How it works

1. **Build werf images** — `generate-pdf.sh` builds three werf images from `docs/documentation`:
   - `website-docs/web/static` — rendered Jekyll documentation site (HTML/CSS/assets);
   - `website-docs/modules-embedded/static-artifact` — built-in module documentation;
   - `website-docs/pdf-builder` — wkhtmltopdf + Python scripts for PDF rendering.

1. **Export content** — static HTML is exported from the built images into a temporary directory via `docker create` + `docker cp`.

1. **Generate PDF** — `docker run` executes `get_pdf_page.py` inside the `pdf-builder` image. The script reads `main.yml` from the current repository (not baked into the image) to build the document structure, then renders HTML chunks with wkhtmltopdf.

1. **Upload to S3** (CI only) — the generated PDFs are uploaded to `s3://<bucket>/deckhouse-web-<env>/<version>/docs-dkp/<lang>/pdf/`.

#### `main.yml` sidebar file

`docs/documentation/_data/sidebars/main.yml` defines the document structure (table of contents). It is mounted into the `pdf-builder` container at runtime (`-v .../main.yml:/app/main.yml:ro`) so that each branch uses its own version of the file without rebuilding the image.

#### Generating PDFs locally

Run from the repository root:

```shell
make docs-generate-pdf
```

Options:

| Variable | Description |
|----------|-------------|
| `DOC_VERSION=X.Y` | Version string shown in PDF headers and on the cover page. Defaults to `latest` on `main`, to the version number on `release-X.Y` branches, and to `dev` on other branches. |
| `BUILD_LANG=ru` | Generate Russian PDFs only. |
| `BUILD_LANG=en` | Generate English PDFs only. |

Example:

```shell
make docs-generate-pdf DOC_VERSION=1.67
```

Local builds use a local Docker registry at `localhost:4999/docs` (started automatically by `make up`).

#### werf image definition

The `pdf-builder` image is defined in `docs/documentation/werf-pdf-builder.inc.yaml`. It is based on `debian-trixie-slim` and includes:

- wkhtmltopdf 0.12.4 (generic Linux build with patched Qt WebKit);
- Python 3 with `beautifulsoup4` and `yaml`;
- DejaVu fonts.

The image contains `get_pdf_page.py`, `toc_style.css`, and `toc_template.xsl` from `tools/docs/pdf/`. The `main.yml` sidebar file is **not** included in the image — it is mounted at runtime.

#### CI workflows

| Workflow | Trigger | Behavior on failure |
|----------|---------|---------------------|
| `build-and-test_pre-release.yml` | Push to `release-*` branch | `continue-on-error: true` — pipeline stays green |
| `docs-pdf-daily.yml` | Daily at 03:00 UTC, or manual `workflow_dispatch` | Fails the job — alerts on broken `main` |

The pre-release workflow runs after `doc_web_build` completes. The daily workflow runs independently on the default branch (`main`) and generates PDFs with `DOC_VERSION=latest`.

Both workflows upload PDFs to the production S3 bucket using secrets `DOC_S3_ACCESS_KEY_ID_PROD`, `DOC_S3_SECRET_ACCESS_KEY_PROD`, `DOC_S3_BUCKET_PROD`, `DOC_S3_REGION`, and `DOC_S3_EP`.
