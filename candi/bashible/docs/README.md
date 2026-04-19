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

* The decision to include a step in the bundle is based on a cloud provider name.
  * cloud-provider - supported cloud providers:
    * aws
    * azure
    * gcp  
    * openstack
    * vsphere
    * yandex
    * dvp

* `runType` - step execution type, which is used during Go templates compilation:
  * ClusterBootstrap - bootstrap first master node
  * Normal - daily bashible execution by schedule

## Steps location

* `bashible/`
  * `bashbooster/` – bashbooster framework directory (it makes writing bashible steps easier)
  * `common-steps/` - common steps for all bundles
    * `all/` - for all run types
    * `cluster-bootstrap/` - only for the `ClusterBootstrap` run type
  * `bashible.sh.tpl` - bashible steps entrypoint
* `cloud-providers/` - cloud-providers list
  * `*cloud_provider_name*/`
    * `bashible/`
      * `common_steps/` – common steps for all bundles
        * `bootstrap-networks.sh.tpl` – if file exists, it will be used instead of a file from a bundle

## How to render bashible bundle?

Bundle compilation is possible with using `dhctl` tool.

```bash
dhctl config render bashible-bundle --config=/config.yaml
```
