# Deckhouse documentation website 

> May contain incomplete information. It is being updated.

This document describes Deckhouse documentation architecture and how to run the documentation website locally.

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
