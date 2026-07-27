---
title: "Application delivery with Argo CD"
permalink: en/admin/configuration/delivery/argocd/
description: "Application delivery with Argo CD in Deckhouse Kubernetes Platform."
lang: en
relatedLinks:
  - title: "Official Argo CD website"
    url: "https://argo-cd.readthedocs.io"
  - title: "Official Argo CD Operator website"
    url: "https://argocd-operator.readthedocs.io"
---

This section describes application delivery with Argo CD in Deckhouse Kubernetes Platform (DKP).

[Argo CD](https://argo-cd.readthedocs.io/en/stable/) is an Open Source tool for continuous delivery of applications to Kubernetes that implements the GitOps approach.
A Git repository is the source of truth for describing applications, their configuration, and target environments.

Argo CD tracks changes, shows differences between the desired and actual resource state,
and when needed brings the cluster to the target state automatically or manually.

Main Argo CD capabilities:

- automatic and manual application deployment in Kubernetes;
- visualization of differences between the desired and actual state;
- tracking application configuration in Git;
- working with multiple Kubernetes clusters;
- access control through its own role model;
- rollback to a previously recorded configuration and action audit;
- working through the web interface, CLI, and API, including integration with Git webhooks and external automation systems.

In DKP, Argo CD is run with the [operator-argo](/modules/operator-argo/) module, based on the [Argo CD Operator](https://argocd-operator.readthedocs.io) project.
The module lets you declaratively deploy and maintain one or more Argo CD instances in a DKP cluster without manually installing and maintaining components.

Additionally, the module provides:

- single sign-on (SSO) support;
- integration with monitoring in DKP;
- high availability mode;
- support for automatic image updates via Argo CD Image Updater;
- access control for cluster resources.
