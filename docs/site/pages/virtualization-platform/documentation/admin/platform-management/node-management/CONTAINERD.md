---
title: "Containerd configuration"
permalink: en/virtualization-platform/documentation/admin/platform-management/node-management/containerd.html
---

## General information

You can configure containerd by creating configuration files using a NodeGroupConfiguration resource.

containerd is configured using the built-in script [`032_configure_containerd.sh`](https://github.com/deckhouse/deckhouse/blob/main/candi/bashible/common-steps/all/032_configure_containerd.sh.tpl).
This script combines all configuration files of the `containerd` service at `/etc/containerd/conf.d/*.toml`
and reboots the service.

When configuring the NodeGroupConfiguration resource, consider the following:

- The `/etc/containerd/conf.d/` directory isn't created automatically.
- You should create files in this directory before running the `032_configure_containerd.sh` script,
  meaning with the priority of less than `32`.

## Additional containerd settings

{% alert level="danger" %}
Adding custom settings results in a reboot of the `containerd` service.
{% endalert %}

{% alert level="warning" %}
You can override parameter values defined in `/etc/containerd/deckhouse.toml`
but it's your responsibility to ensure they work properly.
We recommend avoiding modifications that can potentially affect the master nodes.
{% endalert %}

### Enabling metrics for containerd

Enabling the metrics is the simplest example of how you can add extra settings to the `containerd` service.

Things to note:

1. The script creates a directory with configuration files.
1. The script creates a file in the `/etc/containerd/conf.d` directory.
1. The script has a priority of `31` (`weight: 31`).
1. Configuration on master nodes isn't changed, only on the `worker` group nodes.
1. Metrics collection has to be configured separately. The following example is only for enabling the collection process.
1. The script uses the [`bb-sync-file`](http://www.bashbooster.net/#sync) function of Bash Booster to synchronize the file contents.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-enable-metrics.sh
spec:
  bundles:
    - '*'
  content: |
    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/metrics_settings.toml - << EOF
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

### Adding a private registry with authentication

To run private applications, you may need a private registry that requires authentication to access.
The `containerd` service lets you configure the registry using the `plugins."io.containerd.grpc.v1.cri".registry` parameter.

Provide authentication credentials in the `auth` parameter,
formatted as a Base64-encoded string in the `docker registry auth` format.
To get the Base64-encoded string, run the following command:

```shell
d8 k create secret docker-registry my-secret --dry-run=client --docker-username=User --docker-password=password --docker-server=private.registry.example -o jsonpath="{ .data['\.dockerconfigjson'] }"
eyJhdXRocyI6eyJwcml2YXRlLnJlZ2lzdHJ5LmV4YW1wbGUiOnsidXNlcm5hbWUiOiJVc2VyIiwicGFzc3dvcmQiOiJwYXNzd29yZCIsImF1dGgiOiJWWE5sY2pwd1lYTnpkMjl5WkE9PSJ9fX0=
```

Example of a NodeGroupConfiguration resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
    - '*'
  content: |

    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
          endpoint = ["https://${REGISTRY_URL}"]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
          auth = "eyJhdXRocyI6eyJwcml2YXRlLnJlZ2lzdHJ5LmV4YW1wbGUiOnsidXNlcm5hbWUiOiJVc2VyIiwicGFzc3dvcmQiOiJwYXNzd29yZCIsImF1dGgiOiJWWE5sY2pwd1lYTnpkMjl5WkE9PSJ9fX0="
    EOF
  nodeGroups:
    - "*"
  weight: 31
```

### Adding a certificate for additional registry

<span id="ca-certificate-for-additional-registry"></span>

A private registry may require a root certificate.
Add this certificate to the `/var/lib/containerd/certs` directory
and specify it in the `tls` parameter of the containerd configuration.

Use [guidelines on how to add a root certificate in OS](os.html#adding-ca-certificate) as a template for this script.
Note the following distinctions:

1. The priority value is `31`.
2. The root certificate needs to be added to the `/var/lib/containerd/certs` directory.
3. The certificate path needs to be added to the following configuration settings: `plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls`.

The script uses the following Bash Booster functions:

- [`bb-sync-file`](http://www.bashbooster.net/#sync): To synchronize the file contents.
- [`bb-tmp-file`](http://www.bashbooster.net/#tmp): To create temporary files and delete them once the script has been completed.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-cert-containerd.sh
spec:
  bundles:
  - '*'
  nodeGroups:
  - '*'
  weight: 31
  content: |-
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"
    CERT_CONTENT=$(cat <<"EOF"
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

    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )

    mkdir -p ${CERTS_FOLDER}
    mkdir -p /etc/containerd/conf.d


    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    # Ensure CA certificate file in the CERTS_FOLDER.
    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} 

    # Ensure additional containerd configuration file.
    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE}
```
