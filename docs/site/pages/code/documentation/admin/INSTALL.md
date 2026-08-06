---
title: "Install"
menuTitle: Install
description: How to install a Deckhouse Code instance with the d8code CLI in omnibus and Kubernetes environments
permalink: en/code/documentation/admin/install.html
lang: en
weight: 10
---

`d8code` is the command-line tool that installs a Deckhouse Code instance. It supports two environment types, selected with the `--env-type` flag:

- `omnibus` (default) — installs Deckhouse Code as a package on the local host.
- `k8s` — deploys a Deckhouse Code instance into a Kubernetes cluster.

Every command accepts `--help`.

## Choosing the environment type

`d8code` resolves the environment type from the first source that is set, in this order:

1. The `--env-type` flag.
1. The `D8CODE_ENV_TYPE` environment variable.
1. The `env.type` field in the configuration file.
1. The `omnibus` default.

The environment type determines which arguments are accepted. With `--env-type=k8s`, both the instance `<name>` argument and the `--namespace` flag are required. With `--env-type=omnibus`, neither applies and passing them fails the command.

## Configuration file

`d8code` reads a YAML configuration from `~/.d8code/config`. Override the path with the `D8CODE_CONFIG` environment variable.

```yaml
log:
  format: simple
cache:
  dir: /home/user/.d8code/cache
env:
  type: omnibus
imagesDigests: /path/to/images-digests.json
legacy:
  network: {}
  appConfig: {}
  gitData: {}
  storages: {}
  features: {}
  backup: {}
  scaling:
    targetUserCount: 0
```

The `legacy` section mirrors the `CodeInstance` specification and is required for Kubernetes installations. Validate it before installing:

```shell
d8code config validate
```

Add `--with-kube` to also validate the configuration against the cluster. When the configuration file does not exist, the command prints a notice and exits with code `0`.

## Installing in an omnibus environment

An omnibus installation places Deckhouse Code on the local host using the host package manager.

{% alert level="warning" %}
Installing from a repository is not yet available. Use `--local-file` with a package file you have already downloaded.
{% endalert %}

Install from a local package file:

```shell
sudo d8code install --local-file /path/to/deckhouse-code.deb
```

The `--env-type=omnibus` value is the default, so it can be omitted. The short form of the flag is `-f`.

### Supported distributions

The package format is derived from the file extension and must match the host distribution:

| Package format | Distributions |
| --- | --- |
| `.deb` | Debian, Ubuntu, Astra |
| `.rpm` | RED OS, RHEL, CentOS, AlmaLinux, Rocky, Fedora |

The host distribution is detected from `/etc/os-release`, with redhat-style release files as a fallback. A format mismatch or an unsupported distribution stops the command before any change is made.

### Checks performed before installation

`d8code` runs these checks in order and stops at the first failure:

1. The package file exists at the given path.
1. The file extension is `.deb` or `.rpm`.
1. The detected host distribution uses the same package format.
1. The package name read from the file metadata is `deckhouse-code`. Any other package is rejected.
1. The package is not already installed. If it is, `d8code` prints a warning, suggests the `update` command, and exits with code `0`.

Only the final step — running the package manager — requires superuser privileges, so you can run the checks without `sudo` to validate a package file:

```shell
d8code install --local-file /path/to/deckhouse-code.deb
```

Once the checks pass, `d8code` runs `dpkg -i` for `.deb` packages or `dnf install -y` for `.rpm` packages.

{% alert level="info" %}
If a `.deb` installation fails because of missing dependencies, run `apt-get install -f` and repeat the command.
{% endalert %}

## Installing in a Kubernetes environment

A Kubernetes installation creates a `CodeInstance` from the `legacy` configuration section and applies the resulting objects to the cluster.

### Prerequisites

Prepare the following before you start:

- A configuration file with a filled `legacy` section.
- Container registry credentials, set either with the `--registry-url`, `--registry-username`, and `--registry-password` flags or with the `D8CODE_REGISTRY_URL`, `D8CODE_REGISTRY_USERNAME`, and `D8CODE_REGISTRY_PASSWORD` environment variables. All three values are required.
- An images digests file, set with the `--images-digests` flag, the `D8CODE_IMAGES_DIGESTS` environment variable, or the `imagesDigests` configuration field.
- A defined cluster domain. The installation stops with an error when the cluster domain is empty.

### Running the installation

Preview the objects without changing the cluster:

```shell
d8code install code --env-type=k8s --namespace d8-code --dry-run
```

The `--dry-run` flag prints the objects in YAML instead of applying them. Use it to review the result before the first installation.

Apply the objects:

```shell
d8code install code --env-type=k8s --namespace d8-code
```

The short form of `--namespace` is `-n`. On success the command prints `install complete`.

If the instance is already running in the namespace, `d8code` reports that it is already installed and exits without changes. Use the [update procedure](update.html) for a running instance instead.

Pass the registry credentials explicitly when you do not use environment variables:

```shell
d8code install code \
  --env-type=k8s \
  --namespace d8-code \
  --registry-url registry.example.com \
  --registry-username USERNAME \
  --registry-password PASSWORD \
  --images-digests /path/to/images-digests.json
```

Replace `USERNAME` and `PASSWORD` with the registry credentials, and `registry.example.com` with the registry address.

### Verifying the result

Check the created workloads in the target namespace:

```shell
d8 k -n d8-code get pods
```

## Logging

Both environment types share the logging flags:

```shell
d8code install --local-file /path/to/deckhouse-code.deb --log-format json
```

The `--log-format` flag accepts `simple` (default) or `json` and takes precedence over the `log.format` configuration field. The `--log-color` flag controls colorized `simple` output; it is disabled automatically when the output is not a terminal, and `json` output is never colorized.

To check which version of the tool you are running:

```shell
d8code version
```
