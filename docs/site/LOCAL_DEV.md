# Running a site with the documentation locally

## Requirements

- Clone repo firstly.

- Free 80 port to bind.

- Install [werf](https://werf.io/getting_started/).

## Starting and stopping a site with the documentation locally — the first method

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

## Starting and stopping a site with the documentation locally — the second method

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

There is the `docs/site/werf-debug.yaml` file to compile and the `docs/site/docker-compose-debug.yml` file to run the backend with [delve](https://github.com/go-delve/delve) debugger.

Run from the docs/site folder of the project (or run docs/site/backend/debug.sh):

```shell
werf compose up --config werf-debug.yaml --follow --docker-compose-command-options='-d --force-recreate' --docker-compose-options='-f docker-compose-debug.yml'
```

Connect to localhost:2345

## Working with spellcheck

Use the following commands:
- `make docs-spellcheck` — to check all the documentation for spelling errors.
- `file=<PATH_TO_FILE> make docs-spellcheck` — to check the specified file for spelling errors.

  Example:

  ```shell
  file=ee/se-plus/modules/030-cloud-provider-vsphere/docs/CONFIGURATION_RU.md make docs-spellcheck`
  ```

- `make docs-spellcheck-generate-dictionary` — to generate a dictionary of words. Run it after adding new words to the tools/spelling/wordlist file.
- `make docs-spellcheck-get-typos-list` — to get the sorted list of typos from the documentation.

The `make lint-doc-spellcheck-pr` command is used in CI to check the spelling of the documentation in a PR.
