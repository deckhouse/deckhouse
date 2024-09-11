# Running a site with the documentation locally

- Don't forget to clone repo firstly.

- Free 80 port to bind.

- Install werf

- Open console and start documentation container with the following method:
  - Using makefile:

    ```shell
    cd docs/documentation
    make up 
    ```

    > For development mode use `make dev` instead.

- Open a separate console and start site container with the following method:
  - using makefile:

    ```shell
    cd docs/site
    make up 
    ```

    > For development mode use `make dev` instead.

- Open <http://localhost>.

Don't forget to stop documentation and site containers by running:

```shell
werf compose down
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
  file=ee/modules/030-cloud-provider-vsphere/docs/CONFIGURATION_RU.md make docs-spellcheck`
  ```

- `make docs-spellcheck-generate-dictionary` — to generate a dictionary of words. Run it after adding new words to the tools/spelling/wordlist file.
- `make docs-spellcheck-get-typos-list` — to get the sorted list of typos from the documentation.

The `make lint-doc-spellcheck-pr` command is used in CI to check the spelling of the documentation in a PR.
