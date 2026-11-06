---
title: VM disk data export
description: Writing the contents of a virtual machine disk or of its snapshot to a file so that the data can be moved outside the cluster.
lang: en
---

Export requires the enabled [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) module and, from the command line, the `d8` utility version 0.20.7 or later. The export is performed by the `d8 data export download` command. The [disk](glossary.html#vm-disk) must not be in use at the moment of export, so a machine it is attached to is stopped first. A [snapshot](glossary.html#vm-snapshot) can be exported instead of the disk.

A downloaded file is imported back into the cluster by uploading it as an [image](glossary.html#vm-image) or as a disk.

For details, see [Exporting virtual machine disk data](user/virtualization/data-export.html).
