---
title: "The csi-vsphere module"
---

## The csi-vsphere module

This module allows you to organize disk orders in static clusters
based on vSphere where it is not possible to use the cloud-provider-vsphere module.
For the module to work, virtual machines must be created using
vSphere. The name of the virtual machine in vSphere must match the host name.
The parameter must be enabled in the virtual machine settings ```disk.EnableUUID:TRUE```.
