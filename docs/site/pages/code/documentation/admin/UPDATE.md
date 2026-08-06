---
title: "Update"
menuTitle: Update
description: How to update a Deckhouse Code instance with the d8code CLI in omnibus and Kubernetes environments
permalink: en/code/documentation/admin/update.html
lang: en
weight: 11
---

The `d8code update` command updates an existing Deckhouse Code instance. Like [installation](install.html), it works in two environment types selected with `--env-type`:

- `omnibus` (default) — updates the package on the local host.
- `k8s` — updates an instance in a Kubernetes cluster.

The environment type is resolved from the `--env-type` flag, then the `D8CODE_ENV_TYPE` environment variable, then the `env.type` configuration field, and defaults to `omnibus`. With `--env-type=k8s`, the instance `<name>` argument and the `--namespace` flag are required.

{% alert level="info" %}
`install` and `update` are not interchangeable. `install` refuses to touch an instance that already exists, and `update` refuses to create one that does not. If you pick the wrong command, `d8code` tells you which one to use and exits with code `0` without making changes.
{% endalert %}

## Updating in an omnibus environment

An omnibus update replaces the package on the local host.

{% alert level="warning" %}
Updating from a repository is not yet available. Use `--local-file` with a package file you have already downloaded.
{% endalert %}

Update from a local package file:

```shell
sudo d8code update --local-file /path/to/deckhouse-code.deb
```

The short form of the flag is `-f`. The `--env-type=omnibus` value is the default and can be omitted.

### Checks performed before the update

The same preflight checks as for installation apply, with one difference in the final state check:

1. The package file exists at the given path.
1. The file extension is `.deb` or `.rpm`.
1. The detected host distribution uses the same package format — `.deb` for Debian, Ubuntu, and Astra; `.rpm` for RED OS, RHEL, CentOS, AlmaLinux, Rocky, and Fedora.
1. The package name read from the file metadata is `deckhouse-code`.
1. The package is already installed. If it is not, `d8code` prints a warning, suggests the `install` command, and exits with code `0`.

Only the package manager step needs superuser privileges, so you can run the checks without `sudo`:

```shell
d8code update --local-file /path/to/deckhouse-code.deb
```

Once the checks pass, `d8code` runs `dpkg -i` for `.deb` packages or `dnf upgrade -y` for `.rpm` packages.

{% alert level="info" %}
If a `.deb` update fails because of missing dependencies, run `apt-get install -f` and repeat the command.
{% endalert %}

### Verifying the result

Confirm the installed version after the update:

```shell
d8code version
```

The output has the form `<version>-<commit> (Golang: go<goversion>)`.

## Updating in a Kubernetes environment

A Kubernetes update rebuilds the objects from the current configuration and applies the difference to the cluster. Objects that are no longer part of the instance are removed.

### Before you update

Complete these steps first:

- Update the `legacy` configuration section to the target state.
- Validate the configuration: `d8code config validate --with-kube`.
- Make sure registry credentials and the images digests file are available, as for [installation](install.html).
- Consider enabling [maintenance mode](configuration/maintenance-mode.html) so that users do not make changes while the update runs.

### Running the update

Review the changes before applying them:

```shell
d8code update code --env-type=k8s --namespace d8-code --dry-run
```

Apply the changes:

```shell
d8code update code --env-type=k8s --namespace d8-code
```

On success the command prints `update complete`. Applied objects are labelled with the target version, so a repeated run converges to the same state.

Pass the registry credentials explicitly when you do not use environment variables:

```shell
d8code update code \
  --env-type=k8s \
  --namespace d8-code \
  --registry-url registry.example.com \
  --registry-username USERNAME \
  --registry-password PASSWORD \
  --images-digests /path/to/images-digests.json
```

Replace `USERNAME` and `PASSWORD` with the registry credentials, and `registry.example.com` with the registry address.

### Verifying the result

Watch the workloads roll out in the target namespace:

```shell
d8 k -n d8-code get pods
```

Check the rollout status of a specific deployment:

```shell
d8 k -n d8-code rollout status deployment/code-webservice-default
```

## Migrating from the operator-managed layout

If the instance is still managed by the Deckhouse Code operator, migrate it to the CLI-managed layout with a separate command:

```shell
d8code legacy-migrate --name code --namespace d8-code
```

The migration runs in phases and prompts before each one. To run a single phase, pass `--phase`. To see the planned changes without applying them, add `--dry-run`.

{% alert level="warning" %}
Migration changes the topology of a running instance. Run it with `--dry-run` first and plan a maintenance window.
{% endalert %}
