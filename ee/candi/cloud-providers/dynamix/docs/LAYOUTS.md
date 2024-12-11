---
title: "Cloud provider - Dynamix: Layouts"
description: "Schemes of placement and interaction of resources in Dynamix when working with the Deckhouse cloud provider."
---

Before reading this document, make sure you are familiar with the [Cloud provider layout](/deckhouse/docs/documentation/pages/CLOUD-PROVIDER-LAYOUT.md).

Two layouts are supported.

## Standard

**Recommended layout.**

`Standard` layout is used when you have a preconfigured external network, and you need to allocate public IP address to each instance.

* A separate [resource group](https://registry.terraform.io/providers/rudecs/decort/latest/docs/data-sources/resgroup) is created for the cluster.
* An external IP address is dynamically allocated to each instance.

> **Caution!**
> All applications running on nodes will be available at a public IP address. For example, `kube-apiserver` on master nodes will be available on port 6443. To avoid this, you can restrict access manually.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSMN7OV2eoyx1cqqVK62xq5GrPqi73qcUrUbeRlHwDGbn9x1A7UNAKWGUDpcR7i_Z2W2delx6dIxwjy/pub?w=1000&h=774)
<!--- Source: https://docs.google.com/drawings/d/1EqkEFD68b_yR0DeZNwH_2FQ42P2JAv9eUcPwx9JECww/edit --->

Example of the layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    storageEndpoint: "<storage endpoint>"
    pool: "<pool>"
    externalNetwork: "<external network>"
```

## StandardWithInternalNetwork

`StandardWithInternalNetwork` layout is used when you do not have a preconfigured internal network, and you need to allocate public IP address to each instance.

* A separate [resource group](https://registry.terraform.io/providers/rudecs/decort/latest/docs/data-sources/resgroup) is created for the cluster.
* An external IP address is dynamically allocated to each instance (it is used for Internet access only).
* A separate [ViNS](https://registry.terraform.io/providers/rudecs/decort/latest/docs/data-sources/vins)(internal network) is created for the cluster. You should specify the `nodeNetworkCIDR` and `nameservers` in the configuration.

> **Caution!**
> All applications running on nodes will be available at a public IP address. For example, `kube-apiserver` on master nodes will be available on port 6443. To avoid this, you can restrict access manually.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQpK7j9CTurWuM_XEz02lkFLhk7d46Ur65PWX3vE0mnh-Ccl_C6SxQA3dw2lM-EK8y37ZVs8PVyOqHB/pub?w=812&h=655)
<!--- Source: https://docs.google.com/drawings/d/1Ux-PzrzcNFh-NhLxi_k2pIIRSBMI2ayc9y9zPitVL80/edit --->

Example of the layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: StandardWithInternalNetwork
sshPublicKey: "ssh-rsa AAAA"
location: dynamix
account: acc_user
nodeNetworkCIDR: "10.241.32.0/24"
nameservers: ["10.0.0.10"]
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    storageEndpoint: "<storage endpoint>"
    pool: "<pool>"
    externalNetwork: "<external network>"
```
