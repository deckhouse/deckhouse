---
title: "Platform installation"
permalink: en/virtualization-platform/documentation/admin/install/steps/install.html
---

> When you install Deckhouse Enterprise Edition from the official `registry.deckhouse.io` container registry, you must first log in with your license key:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

The command to pull the installer container from the Deckhouse public registry and run it looks as follows:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

, where:
- `<DECKHOUSE_REVISION>` — [edition](../revision-comparison.html) of Deckhouse (e.g., `ee` for Enterprise Edition, `ce` for Community Edition, etc.);
- `<MOUNT_OPTIONS>` — options for mounting files in the installer container, such as:
  - SSH authentication keys;
  - config file;
  - resource file, etc.
- `<RELEASE_CHANNEL>` — Deckhouse [release channel](../modules/002-deckhouse/configuration.html#parameters-releasechannel) in kebab-case. Should match with the option set in `config.yml`:
  - `alpha` — for the *Alpha* release channel;
  - `beta` — for the *Beta* release channel;
  - `early-access` — for the *Early Access* release channel;
  - `stable` — for the *Stable* release channel;
  - `rock-solid` — for the *Rock Solid* release channel.

Here is an example of a command to run the installer container for Deckhouse CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yml:/resources.yml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

The installation of Deckhouse in the installer container can be started using the `dhctl` command:
- Use the `dhctl bootstrap` command, to start a Deckhouse installation including cluster deployment (these are all cases, except for installation Deckhouse in an existing cluster);
- Use the `dhctl bootstrap-phase install-deckhouse` command, to start a Deckhouse installation in an existing cluster;

> Run `dhctl bootstrap -h` to learn more about the parameters available.

This command will start the Deckhouse installation in a cloud:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml --config=/resources.yml
```

, where:
- `/config.yml` — installation config;
- `/resources.yml` — file with the resource manifests;
- `<SSH_USER>` — SSH user on the server;
- `--ssh-agent-private-keys` — file with the private SSH key for connecting via SSH.