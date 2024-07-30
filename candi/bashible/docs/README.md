---
title: System Configuration Framework - Bashible
---

## Description

Bashible consists of small bash scripts, that are called `steps`.
* Each step describes some task, e.g., cri installation, kubelet installation.

* Steps are executed in the alphabetic order. Because of it, they are named with a number prefix, e.g., `010_step_...`.

* Each step is a `.tpl` file in Go template format. It is required for:
  * Dynamically configuration and updates base on the Kubernetes API
  * Use the same code for different purposes: installation, daily routine, vm image creation

* Step files, and the bashible entrypoint are located in the `/var/lib/bashible` directory.
  * Also, there is a bashible entrypoint for EE version in the `./ee/candi/bashible` directory. 

* Bashible is periodically executing on a Node by systemd unit timer.  

* To make steps cleaner and shorter, bashible framework utilizes an SCM (software configuration management), which is written in the clean bash - [Bash Booster](./candi/bashible/bashbooster).

* The decision to include a step in the bundle is based on a bashible bundle name and a cloud provider name.
  * bashible bundle - bundle which is based on the operating system:
    * ubuntu-lts
    * centos
    * debian
  * cloud-provider - supported cloud providers:
    * aws
    * azure
    * gcp  
    * openstack
    * vsphere
    * yandex

* `runType` - step execution type, which is used during Go templates compilation:
  * ClusterBootstrap - bootstrap first master node
  * Normal - daily bashible execution by schedule
  * ImageBuilding - setup a VM image

## Steps location

* `bashible/`
  * `bashbooster/` – bashbooster framework directory (it makes writing bashible steps easier)
  * `bundles/` – the list of directories, their names are equal to the names of supported bundles
    * `*bundle_name*/`
      * `all/` - for all run types
      * `cluster-bootstrap/` - only for the `ClusterBootstrap` run type
      * `node-group/` - for all run types except the `ImageBuilding`
  * `common-steps/` - common steps for all bundles
    * `all/` - for all run types
    * `cluster-bootstrap/` - only for the `ClusterBootstrap` run type
    * `node-group/` - for all run types except the `ImageBuilding`
  * `bashible.sh.tpl` - bashible steps entrypoint
  * `detect_bundle.sh` - a script to detect bashible bundle, only for the `ClusterBootstrap`
* `cloud-providers/` - cloud-providers list
  * `*cloud_provider_name*/`
    * `bashible/`
      * `bundles/` – additional steps that will included to bundle for cloud installations
        * `*bundle_name*/`
          * `all/` - for all run types
          * `node-group/` - for all run types except the `ImageBuilding`
          * `bootstrap-networks.sh.tpl` – a minimal script to do initials network bootstrap to be able to connect to the Kubernetes API (only for run type `Normal` or `ClusterBootstrap`)
      * `common_steps/` – common steps for all bundles
        * `bootstrap-networks.sh.tpl` – if file exists, it will be used instead of a file from a bundle

## How to render bashible bundle?

Bundle compilation is possible with using `dhctl` tool.

```bash
dhctl config render bashible-bundle --config=/config.yaml
```

Example for `config.yaml`:

```yaml
apiVersion: deckhouse.io/v1
kind: BashibleTemplateData
bundle: ubuntu-lts
provider: OpenStack
runType: ClusterBootstrap
registry:
  host: registry.deckhouse.io
  auth: "test:test"
clusterBootstrap:
  clusterDNSAddress: 10.222.0.10
  clusterDomain: cluster.local
  nodeIP: 192.168.199.23
kubernetesVersion: "1.27"
cri: "Containerd"
nodeGroup:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: master
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
    mainNetwork: shared
    rootDiskSizeInGb: 20
  maxPerZone: 3
  minPerZone: 1
  name: master
  nodeType: CloudEphemeral
  zones:
  - nova
k8s:
  '1.23':
    patch: 10
    bashible:
      ubuntu:
        '18.04':
          containerd:
            desiredVersion: "containerd.io=1.4.6-1"
            allowedPattern: "containerd.io=1.[4]"
```
