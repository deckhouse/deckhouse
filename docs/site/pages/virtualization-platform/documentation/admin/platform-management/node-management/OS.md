---
title: "OS configuration"
permalink: en/virtualization-platform/documentation/admin/platform-management/node-management/os.html
---

## Installing the cert-manager plugin for kubectl on master nodes

You can use NodeGroupConfiguration to install the required utilities on master nodes.

The following example describes installation of the cmctl utility from the cert-manager project.
You can use the same command as a kubectl plugin.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: kubectl-plugin-cert-manager.sh
spec:
  weight: 100
  bundles:
    - "*"
  nodeGroups:
    - "master"
  content: |
    # See https://github.com/cert-manager/cmctl/releases/tag/v2.1.0
    version=v2.1.1

    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cmctl/releases/download/${version}/cmctl_linux_amd64.tar.gz -o - | tar zxf - cmctl
    mv cmctl /usr/local/bin
    ln -s /usr/local/bin/cmctl /usr/local/bin/kubectl-cert_manager
```

## Modifying the sysctl parameters

When performing tasks on nodes, some of them may require you to modify the sysctl parameters.

For example, applications that use mmapfs may require you to increase the allowed number of allocated mappings.
That number is set in the `vm.max_map_count` parameter and can be adjusted using NodeGroupConfiguration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: sysctl-tune.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "worker"
  content: |
    sysctl -w vm.max_map_count=262144
```

## Installing the required kernel version

Some nodes may require a specific version of the Linux kernel to be installed.
In this case, you can use NodeGroupConfiguration.
To simplify a script, we recommend using the [Bash Booster](http://www.bashbooster.net/) functions.

Different operating systems require different operations to modify the kernel version,
so the following are the examples for Debian and CentOS.

Both examples use the `bb-deckhouse-get-disruptive-update-approval` function
as an extended set of Bash Booster commands from the Deckhouse team.
This function prevents a node from rebooting if it must be confirmed by adding an annotation to the node.

Other Bash Booster functions used:

- [`bb-apt-install`](http://www.bashbooster.net/#apt): To install an apt package and send the `bb-package-installed` event when the package is installed.
- [`bb-dnf-install`](http://www.bashbooster.net/#yum): To install a yum package and send the `bb-package-installed` event when the package is installed.
- [`bb-event-on`](http://www.bashbooster.net/#event): To notify about a required node reboot if the `bb-package-installed` event has been sent.
- [`bb-log-info`](http://www.bashbooster.net/#log): For logging.
- [`bb-flag-set`](http://www.bashbooster.net/#flag): To notify that a node reboot is required.

### For Debian-based distributions

Create a NodeGroupConfiguration resource by specifying the desired kernel version in the `desired_version` variable:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

### For CentOS-based distributions

Create a NodeGroupConfiguration resource by specifying the desired kernel version in the `desired_version` variable:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-dnf-install "kernel-${desired_version}"
```

## Adding a root certificate

<span id="adding-ca-certificate"></span>

You might need to add an extra root certificate, for example, to access internal resources of an organization.
You can add a root certificate as a NodeGroupConfiguration resource.

{% alert level="warning" %}
The following example is for Ubuntu OS.
The method of adding certificates to the store may differ depending on the OS.

To adapt the script to a different OS, modify the [`bundles`](../../../../reference/cr/nodegroupconfiguration.html#nodegroupconfiguration-v1alpha1-spec-bundles) and [content](../../../../reference/cr/nodegroupconfiguration.html#nodegroupconfiguration-v1alpha1-spec-content) parameters.
{% endalert %}

The script uses the following Bash Booster functions:

- [`bb-sync-file`](http://www.bashbooster.net/#sync): To synchronize the file contents and send the `ca-file-updated` event if the file has been changed.
- [`bb-event-on`](http://www.bashbooster.net/#event): To initiate the certificate update if the `ca-file-updated` event has been sent.
- [`bb-tmp-file`](http://www.bashbooster.net/#tmp): To create temporary files and delete them once the script has been completed.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca.sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    CERT_FILE_NAME=example_ca
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )

    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates
    }

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
```

A root certificate for containerd is configured in the similar way.
Refer to an example in [Adding a certificate for additional registry](containerd.html#adding-a-certificate-for-additional-registry).
