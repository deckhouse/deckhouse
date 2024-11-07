---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/admin/install/requirements.html
---

## Preparing the infrastructure

Before installing, ensure that:
- the server's OS is in the list of supported OS (or compatible with them) and SSH access to the server with key-based authentication is configured;
- you have access to the container registry with Deckhouse images (default is `registry.deckhouse.io`).

## Supported OS

|Linux distribution|Supported versions|
|------------------|------------------|
|РЕД ОС | 7.3, 8.0|
|РОСА Сервер | 7.9, 12.4, 12.5.1|
|ALT Linux | p10, 10.0, 10.1, 10.2, 11|
|Astra Linux Special Edition | 1.7.2, 1.7.3, 1.7.4, 1.7.5, 1.8|
|CentOS | 7, 8, 9|
|Debian | 10, 11, 12|
|Rocky | Linux 8, 9|
|Ubuntu | 18.04, 20.04, 22.04, 24.04|

## Requirements for storage systems

To ensure the platform operates correctly, you need to install at least one or more storage systems that provide:
- data persistence;
- proper functioning of virtual disks;
- correct operation of the internal Container Registry (DVCR).

To view the list of supported storage systems, please follow the link.