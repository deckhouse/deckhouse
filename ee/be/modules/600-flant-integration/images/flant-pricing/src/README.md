# flant-pricing

## Description

This component collects cluster telemerty metrics, some of which are used for pricing.

## Develop

Requirements:

- Python 3.7+
- pyenv

Install dependencies:

```shell
make deps
```

In `hooks/` directory, place two files: python hook and the hook config, which is a YAML file. Also.
place the hook test under `test` dir. For example:

```shell
$ ls hooks
node_metrics.py
node_metrics.yaml
node_metrics_test.py
```

To test, in `flant-pricing` directory, run:

```shell
make test
```
