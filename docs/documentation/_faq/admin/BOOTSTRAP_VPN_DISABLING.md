---
title: What to do if, with VPN enabled, the container with the installer cannot access the network of the computer from which the cluster bootstrap is being performed?
lang: en
---

If a VPN is installed on the computer from which the cluster bootstrap is performed, there may be a problem with the container with the installer (or graphical installer) accessing the computer's network stack. Because of this, for example, the graphical installer may not be displayed in the browser.

The problem can be solved in one of the following ways:

- Disable VPN on your computer and restart the container with the installer.
- If you cannot disable VPN (for example, if the cluster bootstrap is running on a VPN network), use the `--network host` parameter when starting the container with the installer (**for Docker Desktop on Mac OS, the parameter is available [starting with version 4.34.0](https://docs.docker.com/desktop/release-notes/#4340)**). This will allow the container to access the host's network stack.

  Example of launching a container with an installer on a computer with VPN enabled, using the `--network host` parameter:

  ```shell
  docker run --network host --pull=always -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" -v "$PWD/dhctl-tmp:/tmp/dhctl" registry.deckhouse.ru/deckhouse/ce/install:early-access bash
  ```

  Example of running the graphical installer on a computer with VPN enabled, using the `--network host` parameter:

  ```shell
  docker run --network host --rm --pull always -v $HOME/.d8installer:$HOME/.d8installer -v /var/run/docker.sock:/var/run/docker.sock -p 127.0.0.1:8080:8080 registry.deckhouse.ru/deckhouse/installer:latest -r $HOME/.d8installer
  ```
